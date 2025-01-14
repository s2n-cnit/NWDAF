package subscription

import (
	"errors"
	"github.com/s2n-cnit/nwdaf/pkg/database"
	"github.com/s2n-cnit/nwdaf/pkg/models"
	"github.com/sirupsen/logrus"
)

var SubscriptionMapByNFid = make(map[string][]models.Subscription)  // Used to optimize retrieval when need all sub by a NF
var SubscriptionMapByEvent = make(map[string][]models.Subscription) // Used to optimize retrieval when an event occur, and we need all subscriptions to that event
var SubscriptionMapByNotifCorrId = make(map[string]models.Subscription)

func GetSubscriptions() []models.Subscription {
	var subList []models.Subscription
	for _, subscription := range SubscriptionMapByNotifCorrId {
		subList = append(subList, subscription)
	}
	if subList == nil {
		return []models.Subscription{}
	}
	return subList
}

func GetSubscriptionByNotifCorrId(key string) *models.Subscription {
	subscription, exists := SubscriptionMapByNotifCorrId[key]
	if !exists {
		return nil
	}
	return &subscription
}

func GetSubscriptionByNFid(nfID string) *[]models.Subscription {
	subscription, exists := SubscriptionMapByNFid[nfID]
	if !exists {
		return &[]models.Subscription{}
	}
	return &subscription
}

func GetSubscriptionsByEvent(key string) *[]models.Subscription {
	subscriptionList, exists := SubscriptionMapByEvent[key]
	if !exists {
		return &[]models.Subscription{}
	}
	return &subscriptionList
}

func AddSubscription(subscription models.Subscription, saveInDB bool) (*models.Subscription, error) { //TODO the key should be inside subscription
	//CHECKING that the sub is not already present
	if !(GetSubscriptionByNotifCorrId(subscription.GetNotifCorrId()) == nil) {
		errMsg := "Subscription already exists: " + subscription.GetNotifCorrId()
		logrus.Println(errMsg)
		return nil, errors.New(errMsg)
	}
	subscriptionListByNf := GetSubscriptionByNFid(subscription.GetNFid())
	for _, existingSubscription := range *subscriptionListByNf {
		for _, existingSubEvent := range existingSubscription.GetEvents() {
			for _, subEvent := range subscription.GetEvents() {
				if existingSubEvent.Event == subEvent.Event {
					errMsg := "Warning: Event already subscribed:" + subEvent.Event
					logrus.Println(errMsg)
					return nil, errors.New(errMsg)
				}
			}
		}
	}
	insertSubscription(subscription)
	if saveInDB {
		database.AddMongoDBSubscription(subscription)
	}
	return &subscription, nil
}

func insertSubscription(subscription models.Subscription) {
	// ------------ INSERTING by NF ID
	SubscriptionMapByNFid[subscription.GetNFid()] = append(SubscriptionMapByNFid[subscription.GetNFid()], subscription)
	// ------------ INSERTING by EVENT TYPE
	for _, eventSubscription := range subscription.GetEvents() {
		SubscriptionMapByEvent[eventSubscription.Event] = append(SubscriptionMapByEvent[eventSubscription.Event], subscription)
	}
	// ------------ INSERTING by NOTIFY CORR ID
	SubscriptionMapByNotifCorrId[subscription.GetNotifCorrId()] = subscription
}

func LoadSubscriptionsFromDB() {
	subscriptions := database.GetMongoDBSubscriptions()
	for _, subscription := range subscriptions {
		AddSubscription(subscription, false)
	}
}

func UpdateSubscriptionNotifCorrId(notifCorrId string, subscription models.Subscription) *models.Subscription {
	if GetSubscriptionByNotifCorrId(notifCorrId) == nil {
		SubscriptionMapByNotifCorrId[notifCorrId] = subscription
		return &subscription
	}
	return nil
}

func DeleteSubscriptionNotifCorrId(notifCorrId string) (*models.Subscription, error) {
	if !(GetSubscriptionByNotifCorrId(notifCorrId) == nil) {
		subscription := SubscriptionMapByNotifCorrId[notifCorrId]
		delete(SubscriptionMapByNotifCorrId, notifCorrId)
		delete(SubscriptionMapByNFid, subscription.GetNFid())
		for _, eventType := range subscription.GetEvents() {
			delete(SubscriptionMapByEvent, eventType.Event)
		}
		deletedSub := database.DeleteMongoDBSubscriptionNotifCorrId(notifCorrId)
		if deletedSub == nil {
			return nil, errors.New("Cannot delete sub from database related to: " + notifCorrId)
		}
		return &subscription, nil
	}
	return nil, errors.New("Subscription not found: " + notifCorrId)
}
