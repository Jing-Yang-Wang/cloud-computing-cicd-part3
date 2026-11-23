package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Item struct {
	ID   int    `json:"id" dynamodbav:"id"`
	Name string `json:"name" dynamodbav:"name"`
}

var (
	items = map[int]Item{}
	mu    sync.Mutex

	dynamoClient *dynamodb.Client
	s3Client     *s3.Client
)

func main() {
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
		log.Fatal("Failed to load AWS configuration: ", err)
	}

	dynamoClient = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	http.HandleFunc("/items", itemsHandler)

	log.Println("REST API running on http://localhost:8080")
	log.Println("Connected to LocalStack endpoint: ", endpoint)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func itemsHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		idString := r.URL.Query().Get("id")

		if idString == "" {
			http.Error(w, "Missing id", http.StatusBadRequest)
			return
		}

		itemID, err := strconv.Atoi(idString)
		if err != nil || itemID <= 0 {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}

		result, err := dynamoClient.GetItem(r.Context(), &dynamodb.GetItemInput{
			TableName: aws.String("Items"),
			Key: map[string]types.AttributeValue{
				"id": &types.AttributeValueMemberN{
					Value: strconv.Itoa(itemID),
				},
			},
		})

		if err != nil {
			log.Println("DynamoDB GET error:", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if len(result.Item) == 0 {
			http.Error(w, "Item not found", http.StatusNotFound)
			return
		}

		var item Item
		if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
			http.Error(w, "Failed to decode item", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(item)

	case http.MethodPost:
		var item Item

		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if item.ID <= 0 || item.Name == "" {
			http.Error(w, "Invalid item", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Convert the Go item into a DynamoDB item.
		dbItem, err := attributevalue.MarshalMap(item)
		if err != nil {
			http.Error(w, "Failed to encode item", http.StatusInternalServerError)
			return
		}

		// Create the item only if its ID does not already exist.
		_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String("Items"),
			Item:      dbItem,
			ConditionExpression: aws.String(
				"attribute_not_exists(id)",
			),
		})

		if err != nil {
			var duplicate *types.ConditionalCheckFailedException
			if errors.As(err, &duplicate) {
				http.Error(w, "Item already exists", http.StatusConflict)
				return
			}

			log.Println("DynamoDB error:", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Save the same JSON in S3.
		jsonData, err := json.Marshal(item)
		if err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}

		objectKey := strconv.Itoa(item.ID) + ".json"

		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String("items-bucket"),
			Key:         aws.String(objectKey),
			Body:        bytes.NewReader(jsonData),
			ContentType: aws.String("application/json"),
		})

		if err != nil {
			log.Println("S3 error:", err)

			// Roll back the DynamoDB insert if S3 storage fails.
			_, rollbackErr := dynamoClient.DeleteItem(
				context.Background(),
				&dynamodb.DeleteItemInput{
					TableName: aws.String("Items"),
					Key: map[string]types.AttributeValue{
						"id": &types.AttributeValueMemberN{
							Value: strconv.Itoa(item.ID),
						},
					},
				},
			)
			if rollbackErr != nil {
				log.Println("DynamoDB rollback error:", rollbackErr)
			}

			http.Error(w, "S3 storage error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(item)

	case http.MethodPut:
		var item Item

		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if item.ID <= 0 || item.Name == "" {
			http.Error(w, "Invalid item", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		key := map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberN{
				Value: strconv.Itoa(item.ID),
			},
		}

		// Check that the item already exists.
		existing, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String("Items"),
			Key:       key,
		})

		if err != nil {
			log.Println("DynamoDB GET error:", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if len(existing.Item) == 0 {
			http.Error(w, "Item not found", http.StatusNotFound)
			return
		}

		// Convert the updated item to DynamoDB format.
		dbItem, err := attributevalue.MarshalMap(item)
		if err != nil {
			http.Error(w, "Failed to encode item", http.StatusInternalServerError)
			return
		}

		// Update DynamoDB.
		_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName:           aws.String("Items"),
			Item:                dbItem,
			ConditionExpression: aws.String("attribute_exists(id)"),
		})

		if err != nil {
			var notFound *types.ConditionalCheckFailedException
			if errors.As(err, &notFound) {
				http.Error(w, "Item not found", http.StatusNotFound)
				return
			}

			log.Println("DynamoDB PUT error:", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Update the matching S3 object.
		jsonData, err := json.Marshal(item)
		if err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}

		objectKey := strconv.Itoa(item.ID) + ".json"

		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String("items-bucket"),
			Key:         aws.String(objectKey),
			Body:        bytes.NewReader(jsonData),
			ContentType: aws.String("application/json"),
		})

		if err != nil {
			log.Println("S3 PUT error:", err)
			http.Error(w, "S3 storage error", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(item)

	case http.MethodDelete:
		idString := r.URL.Query().Get("id")

		if idString == "" {
			http.Error(w, "Missing id", http.StatusBadRequest)
			return
		}

		itemID, err := strconv.Atoi(idString)
		if err != nil || itemID <= 0 {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		key := map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberN{
				Value: strconv.Itoa(itemID),
			},
		}

		// Check whether the item exists.
		existing, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String("Items"),
			Key:       key,
		})

		if err != nil {
			log.Println("DynamoDB GET error:", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if len(existing.Item) == 0 {
			http.Error(w, "Item not found", http.StatusNotFound)
			return
		}

		// Delete the corresponding S3 object first.
		objectKey := strconv.Itoa(itemID) + ".json"

		_, err = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String("items-bucket"),
			Key:    aws.String(objectKey),
		})

		if err != nil {
			log.Println("S3 DELETE error:", err)
			http.Error(w, "S3 storage error", http.StatusInternalServerError)
			return
		}

		// Delete the DynamoDB item.
		_, err = dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String("Items"),
			Key:       key,
		})

		if err != nil {
			log.Println("DynamoDB DELETE error:", err)
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
