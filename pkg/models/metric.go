package models

type Metric struct {
	Name        string  `bson:"name"`
	Description string  `bson:"description"`
	Value       float64 `bson:"value"`
	NFid        string  `bson:"nfid"`
	NFType      string  `bson:"nfType"`
}
