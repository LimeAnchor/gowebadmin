package gowebadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

type Sec struct {
	IPAddress string
	Continent string
	Country   string
}

func (m BMAP) Security() (sec Sec) {
	var p Sec
	bsonBytes, err := bson.Marshal(m)
	if err != nil {
		fmt.Println(err)
	}
	bson.Unmarshal(bsonBytes, &p)
	return p
}

type Authority struct {
	Date   primitive.DateTime `json:"date"`
	Result struct {
		Url             string `json:"url"`
		DomainAuthority int    `json:"domainAuthority"`
		PageAuthority   int    `json:"pageAuthority"`
		IsDomain        bool   `json:"isDomain"`
		Message         string `json:"message"`
	} `json:"result"`
	ResultInfo struct {
		Message       string `json:"message"`
		ElapsedTime   int    `json:"elapsedTime"`
		Success       bool   `json:"success"`
		InMaintenance bool   `json:"inMaintenance"`
	} `json:"resultInfo"`
}

type GetAuthority struct {
	Date   time.Time `json:"date"`
	Result struct {
		Url             string `json:"url"`
		DomainAuthority int    `json:"domainAuthority"`
		PageAuthority   int    `json:"pageAuthority"`
		IsDomain        bool   `json:"isDomain"`
		Message         string `json:"message"`
	} `json:"result"`
	ResultInfo struct {
		Message       string `json:"message"`
		ElapsedTime   int    `json:"elapsedTime"`
		Success       bool   `json:"success"`
		InMaintenance bool   `json:"inMaintenance"`
	} `json:"resultInfo"`
}

func (m BMAP) Authority() (auth GetAuthority) {
	var p GetAuthority
	bsonBytes, err := bson.Marshal(m)
	if err != nil {
		fmt.Println(err)
	}
	bson.Unmarshal(bsonBytes, &p)
	return p
}

func (web *WebAdmin) GetAll(collection string, search bson.M) []bson.D {
	mongoClient := web.GetMongoClient()
	con, cancel := context.WithTimeout(context.Background(), 15000*time.Second)
	defer cancel()
	defer mongoClient.Client.Disconnect(con)
	financeDatabase := mongoClient.Client.Database(web.Database.Database)
	col := financeDatabase.Collection(collection)
	defer cancel()
	cur, _ := col.Find(con, search)
	var d []bson.D
	defer cur.Close(con)
	for cur.Next(con) {
		var result bson.D
		cur.Decode(&result)
		d = append(d, result)
	}
	return d
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

func (web *WebAdmin) Upsert(collection string, data interface{}, filter bson.D, upsert bool) {
	mongoclient := web.GetMongoClient()
	con, cancel := context.WithTimeout(context.Background(), 15000*time.Second)
	defer cancel()
	defer mongoclient.Client.Disconnect(con)
	financeDatabase := mongoclient.Client.Database(web.Database.Database)
	col := financeDatabase.Collection(collection)

	var bdoc interface{}
	x, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
	}

	bson.UnmarshalExtJSON(x, true, &bdoc)
	update := bson.D{{"$set", bdoc}}
	opts := options.Update()
	if upsert {
		opts.SetUpsert(true)
	}
	_, err = col.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		fmt.Println(err.Error())
	}
}
