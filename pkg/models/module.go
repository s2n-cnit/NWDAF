package models

type Module struct {
	Name        string `bson:"name"`
	Description string `bson:"description"`
}
