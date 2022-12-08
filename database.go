package gowebadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

func (web *WebAdmin) GetMongoClient() DB {

	serverAPIOptions := options.ServerAPI(options.ServerAPIVersion1)
	clientOptions := options.Client().
		ApplyURI(web.Database.ConnectionString).
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

func (web *WebAdmin) GetOne(collection string, search bson.M) BMAP {
	mongoClient := web.GetMongoClient()
	con, cancel := context.WithTimeout(context.Background(), 15000*time.Second)
	defer cancel()
	defer mongoClient.Client.Disconnect(con)
	financeDatabase := mongoClient.Client.Database(web.Database.Database)
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

func (web *WebAdmin) InsertOne(collection string, data interface{}) error {
	mongoClient := web.GetMongoClient()
	con, cancel := context.WithTimeout(context.Background(), 15000*time.Second)
	defer cancel()
	defer mongoClient.Client.Disconnect(con)
	financeDatabase := mongoClient.Client.Database(web.Database.Database)
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
