# Cloud Computing CI/CD Pipeline (Part 3)

## Project Description

This project implements a RESTful API in Go with persistent storage using Amazon DynamoDB and Amazon S3, simulated locally through LocalStack.

The API supports Create, Read, Update, and Delete (CRUD) operations for items. Each item is stored in both DynamoDB and S3.

The project uses Docker Compose to run the application and its dependencies, automated Go integration tests to verify API responses and storage consistency, and GitHub Actions for continuous integration with Slack notifications.

## Technologies Used

- Go (Golang)
- Docker and Docker Compose
- LocalStack
- Amazon DynamoDB
- Amazon S3
- AWS SDK for Go v2
- GitHub Actions
- Slack Incoming Webhooks

## REST API Endpoints

| Method | Endpoint | Description | Expected Status |
|---|---|---|---|
| GET | `/items?id=1` | Retrieve an existing item | 200 OK |
| POST | `/items` | Create a new item | 201 Created |
| PUT | `/items` | Update an existing item using a JSON body containing its ID | 200 OK |
| DELETE | `/items?id=1` | Delete an existing item | 204 No Content |

The API also returns appropriate error status codes, including 400 for invalid requests, 404 for missing items, and 409 for duplicate item creation.

Example JSON item:

```json
{
  "id": 1,
  "name": "Apple"
}
```

## Architecture

The application uses the following components:

- **Go REST API:** Handles HTTP requests and CRUD operations.
- **DynamoDB:** Stores item records in the `Items` table, using the numeric `id` as the partition key.
- **S3:** Stores each item as a JSON object in the `items-bucket` bucket, using the key `<id>.json`.
- **LocalStack:** Emulates DynamoDB and S3 locally without requiring real AWS resources.
- **Docker Compose:** Coordinates the application, LocalStack, and test environment.

LocalStack initialization scripts automatically create the required DynamoDB table and S3 bucket.

## Prerequisites

- Docker Desktop with Docker Compose
- Git

Go is optional for running the application through Docker, but is useful for local development.

## Running the Application

From the project root directory, start the services:

```bash
docker compose up -d --build
```

The REST API is available at:

`http://localhost:8080/items`

LocalStack is available at:

`http://localhost:4566`

To check the containers:

```bash
docker compose ps
```

To stop the services:

```bash
docker compose down
```

## Testing the REST API

Create an item:

```bash
curl -i -X POST http://localhost:8080/items \
  -H "Content-Type: application/json" \
  -d '{"id":201,"name":"DockerTest"}'
```

Retrieve the item:

```bash
curl -i "http://localhost:8080/items?id=201"
```

Update the item:

```bash
curl -i -X PUT http://localhost:8080/items \
  -H "Content-Type: application/json" \
  -d '{"id":201,"name":"UpdatedItem"}'
```

Delete the item:

```bash
curl -i -X DELETE "http://localhost:8080/items?id=201"
```

## Running Automated Tests

The project includes 10 Go tests covering successful CRUD operations, missing items, invalid requests, and duplicate item creation.

Tests also verify that relevant data in DynamoDB and S3 matches the expected results after CRUD operations.

Run the tests using Docker Compose:

```bash
docker compose run --rm tests
```

All 10 tests should pass when the environment is configured correctly.

## GitHub Actions CI/CD

The workflow is defined in:

`.github/workflows/go-ci.yml`

It supports three triggers:

- Push events
- Pull requests
- Manual execution using `workflow_dispatch`

The workflow checks out the repository, starts LocalStack, runs the automated Go tests using Docker Compose, cleans up the services, and sends a Slack notification.

Workflow runs can be viewed at:

https://github.com/Jing-Yang-Wang/cloud-computing-cicd-part3/actions

## Slack Notifications

The GitHub Actions workflow sends execution status notifications to the Slack `#ci-notifications` channel using an Incoming Webhook.

The webhook URL is stored securely in the GitHub Actions repository secret:

`SLACK_WEBHOOK_URL`

Notifications include the repository name, workflow status, and a link to the GitHub Actions run.

The webhook URL must not be committed to the repository.

## Repository Structure

```text
.github/workflows/go-ci.yml
localstack-init/init.sh
.gitignore
compose.yaml
Dockerfile
Dockerfile.test
go.mod
go.sum
main.go
main_test.go
README.md
run-api.sh
run-tests.sh
```

## Verification

The following checks have been completed:

- LocalStack DynamoDB and S3 initialization
- Dockerized REST API CRUD operations
- DynamoDB and S3 data verification
- All 10 automated Go tests passing locally
- Docker Compose integration tests passing
- GitHub Actions push-triggered workflow succeeding
- GitHub Actions manually triggered workflow succeeding
- Slack success notification delivered to `#ci-notifications`

## Author

Jing-Yang-Wang