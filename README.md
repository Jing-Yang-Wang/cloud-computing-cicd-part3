# Cloud Computing CI/CD Pipeline (Part 2) – Dockerized REST API

## Project Description

This project implements a REST API using Golang and Docker. It demonstrates Continuous Integration (CI) using GitHub Actions.

The API supports GET, POST, PUT, and DELETE operations for managing items.

## Technologies Used

- Golang
- Docker
- GitHub Actions
- Shell Scripts
- Slack Notifications (configured through GitHub Actions)

## REST API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| GET | `/items` | Retrieve all items |
| POST | `/items` | Create an item |
| PUT | `/items?id=1` | Update an existing item |
| DELETE | `/items?id=1` | Delete an item |

## Running the REST API

Make sure Docker is installed and running.

Run:

```bash
sh run-api.sh
```

The REST API will be available at:

http://localhost:8080/items

Press Ctrl+C to stop the API.

## Running Tests

To run the automated tests using Docker:

```bash
sh run-tests.sh
```

The test container exits with status code 0 when all tests pass and a non-zero status when tests fail.

## GitHub Actions CI/CD

The GitHub Actions workflow automatically runs tests when code is pushed or a pull request is created or updated.

Tests can also be triggered manually using the GitHub Actions tab.

The workflow builds a Docker test image and runs the Go unit tests inside the container.

## Slack Notifications

The CI workflow is intended to send a summary of its execution status to a Slack channel. This integration requires a configured Slack webhook and GitHub Actions secret.

## Author

Jing-Yang-Wang