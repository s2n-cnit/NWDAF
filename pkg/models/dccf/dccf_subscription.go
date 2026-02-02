package dccf

import (
	"github.com/s2n-cnit/nwdaf/pkg/models/nwdaf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Package subscription provides functionality related to managing subscriptions made TO and BY the NF importing this package.
// This includes creating, updating, and deleting subscriptions.
// This package info is then used by the event manager to send data to data subscribers that have subscribed and to manage
// notification coming from data sources.
type NotifEndpoint struct {
	NotifUri    string `bson:"notifUri"`
	NotifCorrId string `bson:"notifCorrId"`
}

// Subscription describes an event subscription.
type AnalyticSubscription struct {
	ID             primitive.ObjectID           `bson:"_id,omitempty"`
	AnaNotifUri    string                       `bson:"anaNotifUri"`
	AnaNotifCorrId string                       `bson:"anaNotifCorrId"`
	AnaSub         nwdaf.NwdafEventSubscription `bson:"anaSub"`
	NotifEndpoints []NotifEndpoint              `bson:"notifEndpoints"`
}

func (a AnalyticSubscription) GetBSONFilter(notifCorrId string) bson.D {
	return bson.D{{"anaNotifCorrId", notifCorrId}}
}

func (a AnalyticSubscription) GetNotifCorrId() string {
	return a.AnaNotifCorrId
}

func (a AnalyticSubscription) GetInternalUid() primitive.ObjectID {
	return a.ID
}

func (a AnalyticSubscription) GetNFid() string {
	return a.AnaSub.ConsNfInfo.NfId
}

func (a AnalyticSubscription) GetEvents() []nwdaf.EventSubscription {
	return a.AnaSub.EventSubscriptions
}
