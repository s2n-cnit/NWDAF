package nrf

//
//import (
//	"errors"
//	"go.mongodb.org/mongo-driver/bson/primitive"
//)
//
//// Step 1 and 2: Define custom types and constants
//type NFType string
//
//const (
//	NRF   NFType = "NRF"
//	UDM   NFType = "UDM"
//	AMF   NFType = "AMF"
//	SMF   NFType = "SMF"
//	AUSF  NFType = "AUSF"
//	NEF   NFType = "NEF"
//	PCF   NFType = "PCF"
//	SMSF  NFType = "SMSF"
//	NSSF  NFType = "NSSF"
//	UDR   NFType = "UDR"
//	LMF   NFType = "LMF"
//	GMLC  NFType = "GMLC"
//	EIR5G NFType = "5G_EIR"
//	SEPP  NFType = "SEPP"
//	UPF   NFType = "UPF"
//	N3IWF NFType = "N3IWF"
//	AF    NFType = "AF"
//	UDSF  NFType = "UDSF"
//	BSF   NFType = "BSF"
//	CHF   NFType = "CHF"
//	NWDAF NFType = "NWDAF"
//	PCSCF NFType = "PCSCF"
//	// ... Add other constants ...
//)
//
//type NFStatus string
//
//const (
//	REGISTERED     NFStatus = "REGISTERED"
//	SUSPENDED      NFStatus = "SUSPENDED"
//	UNDISCOVERABLE NFStatus = "UNDISCOVERABLE"
//	CANARY_RELEASE NFStatus = "CANARY_RELEASE"
//)
//
//func (nt NFType) IsValid() error {
//	switch nt {
//	case NRF, UDM, AMF, SMF, AUSF, NEF, PCF, SMSF, NSSF, UDR, LMF, GMLC, EIR5G, SEPP, UPF, N3IWF, AF, UDSF, BSF, CHF, NWDAF, PCSCF:
//		return nil
//	}
//	return errors.New("invalid NFType")
//}
//
//func (ns NFStatus) IsValid() error {
//	switch ns {
//	case REGISTERED, SUSPENDED, UNDISCOVERABLE, CANARY_RELEASE:
//		return nil
//	}
//	return errors.New("invalid NFStatus")
//}
//
//type DccfInfo struct {
//	ServingNfTypeList []NFType `bson:"servingNfTypeList"`
//}
//
//type NFProfile struct {
//	ID             primitive.ObjectID `bson:"_id,omitempty"`
//	NfInstanceId   string             `bson:"nfInstanceId"`
//	NfInstanceName string             `bson:"nfInstanceName"`
//	NfType         NFType             `bson:"nfType"`
//	NfStatus       NFStatus           `bson:"nfStatus"`
//	Ipv4Addresses  []string           `bson:"ipv4Addresses"`
//	Ipv6Addresses  []string           `bson:"ipv6Addresses"`
//	DccfInfo       DccfInfo           `bson:"dccfInfo"`
//	NwdafInfo      AmfInfo            `bson:"nwdafInfo"`
//	AdrfInfoList   []AdrfInfo         `bson:"adrfInfoList"`
//}
