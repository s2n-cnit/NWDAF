package models

import (
	"errors"
	"reflect"

	"github.com/s2n-cnit/nwdaf/pkg/models/dccf"
	"github.com/s2n-cnit/nwdaf/pkg/models/nwdaf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Subscription interface {
	GetBSONFilter(notifCorrId string) bson.D
	GetNotifCorrId() string
	GetInternalUid() primitive.ObjectID
	GetNFid() string
	GetEvents() []nwdaf.EventSubscription
}

type SubscriptionType int

const (
	NwdafEventSubscription SubscriptionType = iota
	DccfAnalyticSubscription
)

var subscriptionTypes = map[SubscriptionType]reflect.Type{
	NwdafEventSubscription:   reflect.TypeOf(nwdaf.EventSubscriptionNWDAF{}),
	DccfAnalyticSubscription: reflect.TypeOf(dccf.AnalyticSubscription{}),
}

func BuildSubscriptionModel(subType SubscriptionType) (Subscription, error) {
	if t, ok := subscriptionTypes[subType]; ok {
		return reflect.New(t).Interface().(Subscription), nil
	}
	return nil, errors.New("unknown subscription type")
}
