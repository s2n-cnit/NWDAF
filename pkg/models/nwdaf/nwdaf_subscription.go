package nwdaf

type ConsumerNfInformation struct {
	NfId    string `json:"nfId"`
	NfSetId string `json:"nfSetId"`
	TaiList string `json:"taiList"`
}

type NwdafEvent struct {
	Description string `json:"description"`
}

type EventSubscription struct {
	AnySlice bool   `json:"anySlice"`
	Event    string `json:"event"`
}

type NwdafEventSubscription struct {
	EventSubscriptions []EventSubscription   `json:"eventSubscriptions"` //TODO create model
	ConsNfInfo         ConsumerNfInformation `json:"consNfInfo"`
	NotifCorrId        string                `json:"notifCorrId"`
	NotificationURI    string                `json:"notificationURI"`
}
