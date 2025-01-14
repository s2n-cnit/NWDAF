package database

import (
	"context"
	"errors"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

const MongoDBContextTimeout = 30 * time.Second

var (
	operatorInitialized bool
	subType             models.SubscriptionType
)

func InitializeSubscriptionOperator(subscriptionType models.SubscriptionType) {
	operatorInitialized = true
	subType = subscriptionType
}

// Add a subscription to MongoDB
func AddMongoDBSubscription(subscription models.Subscription) *models.Subscription {
	ctx, cancel := context.WithTimeout(context.Background(), MongoDBContextTimeout)
	defer cancel()

	// Convert subscription object to BSON // Assuming you have an `ID` field of type `primitive.ObjectID`
	_, err := collection.InsertOne(ctx, subscription)
	if err != nil {
		logrus.Error("Error adding subscription:", err)
		return nil
	}
	return &subscription
}

// Get a subscription by Notification Correlation ID from MongoDB
func GetMongoDBSubscriptionByNotifCorrId(notifCorrId string, subTypeName string) *models.Subscription {
	//TODO DEPENDS on the subscription type cannot use the same filter
	return nil
}

// Update a subscription in MongoDB
func UpdateMongoDBSubscriptionNotifCorrId(notifCorrId string, subscription models.Subscription) *models.Subscription {
	//TODO DEPENDS on the subscription type cannot use the same filter
	return nil
}

// Delete a subscription by Notification Correlation ID from MongoDB
func DeleteMongoDBSubscriptionNotifCorrId(notifCorrId string) *models.Subscription {
	if !operatorInitialized {
		logrus.Fatal("Subscription operator has not been initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	//We need the model to fill the deleted data from DB and to get the filter to identify the item in the collection
	submodel, err := models.BuildSubscriptionModel(subType) //bson.D{{"ananotifcorrid", notifCorrId}}
	if err != nil {
		logrus.Fatal("Error retrieving model:", subType)
	}

	filter := submodel.GetBSONFilter(notifCorrId)
	err = collection.FindOneAndDelete(ctx, filter).Decode(submodel)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			logrus.Error("No documents found with the specified filter. Nothing was deleted.")
			return nil
		} else {
			logrus.Fatalf("Error occurred while deleting document: %v", err)
		}

	}
	return &submodel
}

func GetMongoDBSubscriptions() []models.Subscription {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var subscriptions []models.Subscription

	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		logrus.Error("Error finding subscriptions:", err)
		return nil
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		//Define the model to be filled when deserializing the data from mongo DB
		subModel, err := models.BuildSubscriptionModel(subType)
		if err != nil {
			logrus.Fatal("Error creating new subscription:", err)
		}
		if err := cursor.Decode(subModel); err != nil {
			logrus.Fatal("Error decoding subscription:", err)
		}
		subscriptions = append(subscriptions, subModel)
	}
	return subscriptions
}
