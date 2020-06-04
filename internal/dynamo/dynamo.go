package dynamo

import (
	"errors"
	"time"
	"webchat/internal/config"
	"webchat/internal/data"
	"webchat/internal/logger"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

var (
	db        *dynamodb.DynamoDB
	tableName string // TableName for the name of data
	log       *logger.Logger
)

func init() {
	log = logger.NewLogger("dynamo")
}

// NewDatabase is initiate the SQL database
func NewDatabase(cfg config.DynamoConfig) error {
	// Create database
	log.I("start newsession toward: %v", cfg.Endpoint)
	sess, sessErr := session.NewSession(&aws.Config{
		Region:   aws.String(cfg.Region),
		Endpoint: aws.String(cfg.Endpoint),
		Retryer: client.DefaultRetryer{
			NumMaxRetries:    2,
			MinRetryDelay:    0,
			MinThrottleDelay: 0,
			MaxRetryDelay:    60 * time.Second,
			MaxThrottleDelay: 0,
		},
	})
	if sessErr != nil {
		log.E("%v", sessErr.Error())
		return sessErr
	}

	db = dynamodb.New(sess)

	// Create table Movies
	tableName = "Profiles"
	input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("UID"),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("UID"),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(10),
			WriteCapacityUnits: aws.Int64(10),
		},
		TableName: aws.String(tableName),
	}

	_, err := db.CreateTable(input)
	if err != nil {
		log.E("Got error calling CreateTable: %v", err.Error())
		return nil
	}

	log.I("Created the table %v", tableName)

	log.I("Successfully connected to Dynamo database: %v", cfg.Endpoint)

	return nil
}

// Write is to write an item to dynamodb
func Write(item data.UserProfile) error {
	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		return err
	}

	Input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(tableName),
	}

	_, err = db.PutItem(Input)
	if err != nil {
		return err
	}

	log.I("Successfully write the item: %-v", av)

	return nil
}

// Read is to retrive an item from dynamodb
func Read(uid string) (data.UserProfile, error) {
	var item data.UserProfile

	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"UID": {
				S: aws.String(uid),
			},
		},
	})
	if err != nil {
		log.D("fail to read : %v", err.Error())
		return item, err
	}

	if len(result.Item) == 0 {
		return item, errors.New("No result on query")
	}

	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		log.D("Failed to unmarshal Record, %v", err.Error())
		return item, err
	}

	log.I("Successfully quaried in database: %+v", item)

	return item, nil
}
