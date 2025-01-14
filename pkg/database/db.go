package database

import (
	"context"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB configuration settings
const (
	databaseName = "nwdaf"
)

var client *mongo.Client
var collection *mongo.Collection

// Initialize MongoDB client
func InitMongoDB(mongoURI string, username string, password string, collectionName string) {
	var err error
	clientOptions := options.Client().ApplyURI(mongoURI)
	// Set the username and password for authentication
	if username != "" && password != "" {
		credential := options.Credential{
			Username: username,
			Password: password,
		}
		clientOptions.SetAuth(credential)
	}
	client, err = mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		logrus.Fatal(err)
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		logrus.Fatal(err)
	}

	collection = client.Database(databaseName).Collection(collectionName)
	logrus.Debug("Connected to MongoDB!")
}

// Close the MongoDB client connection
func CloseMongoDB() {
	if err := client.Disconnect(context.TODO()); err != nil {
		logrus.Fatal(err)
	}
	logrus.Debug("Connection to MongoDB closed.")
}
