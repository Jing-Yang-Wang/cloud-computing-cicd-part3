package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

func sendRequest(method, url, body string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}

	req := httptest.NewRequest(method, url, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	itemsHandler(rec, req)
	return rec
}

func checkStorageMatch(t *testing.T, expected Item) {
	t.Helper()

	ctx := context.Background()

	// Read the item directly from DynamoDB.
	dbResult, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("Items"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberN{
				Value: strconv.Itoa(expected.ID),
			},
		},
	})
	if err != nil {
		t.Fatalf("DynamoDB GET failed: %v", err)
	}

	if len(dbResult.Item) == 0 {
		t.Fatal("Item not found in DynamoDB")
	}

	var dbItem Item
	if err := attributevalue.UnmarshalMap(dbResult.Item, &dbItem); err != nil {
		t.Fatal(err)
	}

	// Read the matching JSON object directly from S3.
	s3Result, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("items-bucket"),
		Key:    aws.String(strconv.Itoa(expected.ID) + ".json"),
	})
	if err != nil {
		t.Fatalf("S3 GET failed: %v", err)
	}
	defer s3Result.Body.Close()

	data, err := io.ReadAll(s3Result.Body)
	if err != nil {
		t.Fatal(err)
	}

	var s3Item Item
	if err := json.Unmarshal(data, &s3Item); err != nil {
		t.Fatal(err)
	}

	// Verify both storage locations contain the expected item.
	if dbItem != expected {
		t.Fatalf("DynamoDB mismatch: got %+v, expected %+v", dbItem, expected)
	}

	if s3Item != expected {
		t.Fatalf("S3 mismatch: got %+v, expected %+v", s3Item, expected)
	}
}

func checkStorageDeleted(t *testing.T, id int) {
	t.Helper()

	ctx := context.Background()
	idString := strconv.Itoa(id)

	// Check DynamoDB.
	dbResult, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String("Items"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberN{
				Value: idString,
			},
		},
	})
	if err != nil {
		t.Fatalf("DynamoDB GET failed: %v", err)
	}

	if len(dbResult.Item) != 0 {
		t.Fatalf("Item %d still exists in DynamoDB", id)
	}

	// Check S3.
	_, err = s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String("items-bucket"),
		Key:    aws.String(idString + ".json"),
	})

	if err == nil {
		t.Fatalf("Item %d still exists in S3", id)
	}

	var responseError smithy.APIError

	if !errors.As(err, &responseError) {
		t.Fatalf("S3 check failed: %v", err)
	}

	switch responseError.ErrorCode() {
	case "404", "NotFound", "NoSuchKey":
		// Expected: the S3 object has been deleted.
	default:
		t.Fatalf("Unexpected S3 error: %v", err)
	}
}

func cleanupTestItem(t *testing.T, id int) {
	t.Helper()

	ctx := context.Background()
	idString := strconv.Itoa(id)

	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String("Items"),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberN{
				Value: idString,
			},
		},
	})
	if err != nil {
		t.Fatalf("DynamoDB cleanup failed: %v", err)
	}

	_, err = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String("items-bucket"),
		Key:    aws.String(idString + ".json"),
	})
	if err != nil {
		t.Fatalf("S3 cleanup failed: %v", err)
	}
}

func TestGETExisting(t *testing.T) {
	cleanupTestItem(t, 101)

	t.Cleanup(func() {
		cleanupTestItem(t, 101)
	})

	sendRequest(http.MethodPost, "/items", `{"id":101,"name":"Apple"}`)

	rec := sendRequest(http.MethodGet, "/items?id=101", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var item Item
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}

	if item.ID != 101 || item.Name != "Apple" {
		t.Fatalf("Unexpected item: %+v", item)
	}

	checkStorageMatch(t, Item{
		ID:   101,
		Name: "Apple",
	})
}

func TestGETNotFound(t *testing.T) {
	rec := sendRequest(http.MethodGet, "/items?id=99999", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", rec.Code)
	}
}

func TestGETMissingID(t *testing.T) {
	rec := sendRequest(http.MethodGet, "/items", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", rec.Code)
	}
}

func TestGETInvalidID(t *testing.T) {
	rec := sendRequest(http.MethodGet, "/items?id=abc", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", rec.Code)
	}
}

func TestPOST(t *testing.T) {
	ctx := context.Background()

	// Clean up test data before running.
	cleanup := func() {
		_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String("Items"),
			Key: map[string]types.AttributeValue{
				"id": &types.AttributeValueMemberN{
					Value: "102",
				},
			},
		})
		if err != nil {
			t.Errorf("DynamoDB cleanup failed: %v", err)
		}

		_, err = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String("items-bucket"),
			Key:    aws.String("102.json"),
		})
		if err != nil {
			t.Errorf("S3 cleanup failed: %v", err)
		}
	}

	// Remove leftover data from previous test runs.
	cleanup()

	// Also clean up when this test finishes.
	t.Cleanup(cleanup)

	rec := sendRequest(http.MethodPost, "/items",
		`{"id":102,"name":"Banana"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	checkStorageMatch(t, Item{
		ID:   102,
		Name: "Banana",
	})
}

func TestPOSTDuplicate(t *testing.T) {
	// Clean up data left by previous runs.
	cleanupTestItem(t, 103)

	// Clean up again after this test.
	t.Cleanup(func() {
		cleanupTestItem(t, 103)
	})

	// First POST should succeed.
	first := sendRequest(http.MethodPost, "/items",
		`{"id":103,"name":"Orange"}`)

	if first.Code != http.StatusCreated {
		t.Fatalf("First POST: expected 201, got %d: %s",
			first.Code, first.Body.String())
	}

	// Verify the first POST saved matching data.
	checkStorageMatch(t, Item{
		ID:   103,
		Name: "Orange",
	})

	// Second POST with the same ID should fail.
	second := sendRequest(http.MethodPost, "/items",
		`{"id":103,"name":"Changed"}`)

	if second.Code != http.StatusConflict {
		t.Fatalf("Duplicate POST: expected 409, got %d",
			second.Code)
	}

	// The failed duplicate POST must not change either storage.
	checkStorageMatch(t, Item{
		ID:   103,
		Name: "Orange",
	})
}

func TestPUTExisting(t *testing.T) {
	// Clean up old test data.
	cleanupTestItem(t, 104)

	t.Cleanup(func() {
		cleanupTestItem(t, 104)
	})

	// Create the original item.
	first := sendRequest(http.MethodPost, "/items",
		`{"id":104,"name":"Before"}`)

	if first.Code != http.StatusCreated {
		t.Fatalf("POST: expected 201, got %d: %s",
			first.Code, first.Body.String())
	}

	// Verify original data in DynamoDB and S3.
	checkStorageMatch(t, Item{
		ID:   104,
		Name: "Before",
	})

	// Update the item.
	rec := sendRequest(http.MethodPut, "/items",
		`{"id":104,"name":"After"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: expected 200, got %d: %s",
			rec.Code, rec.Body.String())
	}

	// Verify the updated item through GET.
	getRec := sendRequest(http.MethodGet, "/items?id=104", "")

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET: expected 200, got %d", getRec.Code)
	}

	var item Item
	if err := json.Unmarshal(getRec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}

	if item.ID != 104 || item.Name != "After" {
		t.Fatalf("Unexpected updated item: %+v", item)
	}

	// Verify updated data in both storage locations.
	checkStorageMatch(t, Item{
		ID:   104,
		Name: "After",
	})
}

func TestPUTNotFound(t *testing.T) {
	rec := sendRequest(http.MethodPut, "/items",
		`{"id":99998,"name":"Missing"}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", rec.Code)
	}
}

func TestDELETEExisting(t *testing.T) {
	// Clean up data left by previous runs.
	cleanupTestItem(t, 105)

	// Ensure cleanup even if the test fails.
	t.Cleanup(func() {
		cleanupTestItem(t, 105)
	})

	// Create an item to delete.
	first := sendRequest(http.MethodPost, "/items",
		`{"id":105,"name":"DeleteMe"}`)

	if first.Code != http.StatusCreated {
		t.Fatalf("POST: expected 201, got %d: %s",
			first.Code, first.Body.String())
	}

	// Verify the item exists in both storage locations.
	checkStorageMatch(t, Item{
		ID:   105,
		Name: "DeleteMe",
	})

	// Delete the item.
	rec := sendRequest(http.MethodDelete, "/items?id=105", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE: expected 204, got %d: %s",
			rec.Code, rec.Body.String())
	}

	// GET should return 404 after deletion.
	getRec := sendRequest(http.MethodGet, "/items?id=105", "")

	if getRec.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE: expected 404, got %d",
			getRec.Code)
	}

	// Verify the item is absent from both DynamoDB and S3.
	checkStorageDeleted(t, 105)
}

func TestDELETENotFound(t *testing.T) {
	rec := sendRequest(http.MethodDelete, "/items?id=99997", "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", rec.Code)
	}
}

func TestMain(m *testing.M) {
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	if endpoint == "" {
		endpoint = "http://localhost:4566"
	}

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		),
	)
	if err != nil {
		panic(err)
	}

	dynamoClient = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	os.Exit(m.Run())
}
