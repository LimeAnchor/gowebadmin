package gowebadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"time"
)

type DB struct {
	Client *mongo.Client
}

type BLIST struct {
	Elemennts []BMAP
}
type BMAP bson.M

func (m BMAP) Customer() (profile Customer) {
	var p Customer
	bsonBytes, err := bson.Marshal(m)
	if err != nil {
		fmt.Println(err)
	}
	bson.Unmarshal(bsonBytes, &p)
	return p
}

func GetMongoClient() DB {
	serverAPIOptions := options.ServerAPI(options.ServerAPIVersion1)
	clientOptions := options.Client().
		ApplyURI(os.Getenv("MONGODB_CONNECTION_STRING")).
		SetServerAPIOptions(serverAPIOptions)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mongoClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		fmt.Println(err.Error())
	}
	return DB{
		mongoClient,
	}
}

func GetOne(collection string, search bson.M) BMAP {
	mongoClient := GetMongoClient()
	con, cancel := context.WithTimeout(context.Background(), 15000*time.Second)
	defer cancel()
	defer mongoClient.Client.Disconnect(con)
	financeDatabase := mongoClient.Client.Database(os.Getenv("MONGODB_DATABASE"))
	col := financeDatabase.Collection(collection)
	defer cancel()
	result := col.FindOne(con, search)

	var d BMAP
	err := result.Decode(&d)
	fmt.Println(d)
	if err != nil {
		fmt.Println(err.Error())
	}
	return d
}

func InsertOne(collection string, data interface{}) error {
	mongoClient := GetMongoClient()
	con, cancel := context.WithTimeout(context.Background(), 15000*time.Second)
	defer cancel()
	defer mongoClient.Client.Disconnect(con)
	financeDatabase := mongoClient.Client.Database(os.Getenv("MONGODB_DATABASE"))
	col := financeDatabase.Collection(collection)
	defer cancel()
	x, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)
	}
	var bdoc interface{}
	bson.UnmarshalExtJSON(x, true, &bdoc)
	_, err = col.InsertOne(con, bdoc)
	if err != nil {
		fmt.Println(err)
	}
	return err
}
