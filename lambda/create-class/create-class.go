package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/jbaikge/gocms"
)

var (
	dynamoConfig aws.Config
	resources gocms.DynamoDBResources
)

func HandleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (response events.APIGatewayProxyResponse, err error) {
	var class gocms.Class
	err = json.NewDecoder(strings.NewReader(request.Body)).Decode(&class)
	if err != nil {
		return
	}

	repo := gocms.NewDynamoDBRepository(dynamoConfig, resources)
	service := gocms.NewClassService(repo)

	if err = service.Create(context.Background(), &class); err != nil {
		return
	}

	body, err := json.Marshal(&class)
	if err != nil {
		return
	}

	response.Headers = map[string]string{
		"Content-Type":                "application/json",
		"Access-Control-Allow-Origin": "*",
	}
	response.StatusCode = http.StatusCreated
	response.Body = string(body)
	return
}

func main() {
	var err error
	dynamoConfig, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load default config: %v", err)
	}

	resources.FromEnv()

	lambda.Start(HandleRequest)
}
