package datasource

import (
	"context"
	"errors"
	"fmt"
	wst "github.com/fredyk/westack-go/westack/common"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"regexp"
	"time"
)

type OperationError struct {
	Code    int
	Message string
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("%v %v", e.Code, e.Message)
}

func NewError(code int, message string) *OperationError {
	res := &OperationError{
		code, message,
	}
	return res
}

type Datasource struct {
	Name  string
	Db    interface{}
	Viper *viper.Viper

	Key         string
	Context     context.Context
	ctxCancelFn context.CancelFunc
}

func (ds *Datasource) Initialize() error {
	var connector = ds.Viper.GetString(ds.Key + ".connector")
	switch connector {
	case "mongodb":
		mongoCtx, cancelFn := context.WithCancel(ds.Context)

		dsViper := ds.Viper

		var clientOpts *options.ClientOptions

		url := ""
		if dsViper.GetString(ds.Key+".url") != "" {
			url = dsViper.GetString(ds.Key + ".url")
		} else {
			port := 0
			if dsViper.GetInt(ds.Key+".port") > 0 {
				port = dsViper.GetInt(ds.Key + ".port")
			}
			url = fmt.Sprintf("mongodb://%v:%v/%v", dsViper.GetString(ds.Key+".host"), port, dsViper.GetString(ds.Key+".database"))
			log.Printf("Using composed url %v\n", url)
		}

		if dsViper.GetString(ds.Key+".username") != "" && dsViper.GetString(ds.Key+".password") != "" {
			credential := options.Credential{
				Username: dsViper.GetString(ds.Key + ".username"),
				Password: dsViper.GetString(ds.Key + ".password"),
			}
			clientOpts = options.Client().ApplyURI(url).SetAuth(credential)
		} else {
			clientOpts = options.Client().ApplyURI(url)
		}

		clientOpts = clientOpts.SetSocketTimeout(time.Second * 30).SetConnectTimeout(time.Second * 30).SetServerSelectionTimeout(time.Second * 30).SetMinPoolSize(1).SetMaxPoolSize(5)

		db, err := mongo.Connect(mongoCtx, clientOpts)
		if err != nil {
			cancelFn()
			return err
		}
		ds.Db = db

		err = ds.Db.(*mongo.Client).Ping(mongoCtx, readpref.SecondaryPreferred())
		if err != nil {
			cancelFn()
			return err
		}

		init := time.Now().UnixMilli()
		go func() {
			for {
				time.Sleep(time.Second * 5)

				mongoCtx, cancelFn = context.WithCancel(mongoCtx)
				err := ds.Db.(*mongo.Client).Ping(mongoCtx, readpref.SecondaryPreferred())
				if err != nil {
					log.Printf("Reconnecting %v...\n", url)
					db, err := mongo.Connect(mongoCtx, clientOpts)
					if err != nil {
						cancelFn()
						log.Fatalf("Could not reconnect %v: %v\n", url, err)
					} else {
						err = ds.Db.(*mongo.Client).Ping(mongoCtx, readpref.SecondaryPreferred())
						if err != nil {
							cancelFn()
							log.Fatalf("Mongo client disconnected after %vms: %v", time.Now().UnixMilli()-init, err)
						}

						log.Printf("successfully reconnected to %v\n", url)
					}
					ds.Db = db
				}
			}
		}()
	default:
		return errors.New("Invalid connector " + connector)
	}
	return nil
}

func (ds *Datasource) FindMany(collectionName string, lookups *wst.A) (*wst.A, error) {
	var connector = ds.Viper.GetString(ds.Key + ".connector")
	switch connector {
	case "mongodb":
		var db = ds.Db.(*mongo.Client)

		database := db.Database(ds.Viper.GetString(ds.Key + ".database"))
		collection := database.Collection(collectionName)

		pipeline := wst.A{}

		if lookups != nil {
			pipeline = append(pipeline, *lookups...)
		}
		allowDiskUse := true
		ctx := ds.Context
		cursor, err := collection.Aggregate(ctx, pipeline, &options.AggregateOptions{
			AllowDiskUse: &allowDiskUse,
		})
		if err != nil {
			return nil, err
		}
		defer func(cursor *mongo.Cursor, ctx context.Context) {
			err := cursor.Close(ctx)
			if err != nil {
				panic(err)
			}
		}(cursor, ctx)
		var documents wst.A
		err = cursor.All(ds.Context, &documents)
		if err != nil {
			return nil, err
		}
		return &documents, nil
	}
	return nil, errors.New(fmt.Sprintf("invalid connector %v", connector))
}

func (ds *Datasource) FindById(collectionName string, id interface{}, lookups *wst.A) (*wst.M, error) {
	var _id interface{}
	switch id.(type) {
	case string:
		var err error
		_id, err = primitive.ObjectIDFromHex(id.(string))
		if err != nil {
			_id = id
		}
	default:
		_id = id
	}
	return findByObjectId(collectionName, _id, ds, lookups)
}

func findByObjectId(collectionName string, _id interface{}, ds *Datasource, lookups *wst.A) (*wst.M, error) {
	wrappedLookups := &wst.A{
		{
			"$match": wst.M{
				"_id": _id,
			},
		},
	}
	if lookups != nil {
		*wrappedLookups = append(*wrappedLookups, *lookups...)
	}
	results, err := ds.FindMany(collectionName, wrappedLookups)
	if err != nil {
		return nil, err
	}
	if results != nil && len(*results) > 0 {
		return &(*results)[0], nil
	} else {
		return nil, errors.New("document not found")
	}
}

func (ds *Datasource) Create(collectionName string, data *wst.M) (*wst.M, error) {
	var connector = ds.Viper.GetString(ds.Key + ".connector")
	switch connector {
	case "mongodb":
		var db = ds.Db.(*mongo.Client)

		database := db.Database(ds.Viper.GetString(ds.Key + ".database"))
		collection := database.Collection(collectionName)
		insertOneResult, err := collection.InsertOne(ds.Context, data)
		if err != nil {
			return nil, err
		}
		return findByObjectId(collectionName, insertOneResult.InsertedID, ds, nil)
	}
	return nil, errors.New(fmt.Sprintf("invalid connector %v", connector))
}

func (ds *Datasource) UpdateById(collectionName string, id interface{}, data *wst.M) (*wst.M, error) {
	var connector = ds.Viper.GetString(ds.Key + ".connector")
	switch connector {
	case "mongodb":
		var db = ds.Db.(*mongo.Client)

		database := db.Database(ds.Viper.GetString(ds.Key + ".database"))
		collection := database.Collection(collectionName)
		delete(*data, "id")
		delete(*data, "_id")
		if _, err := collection.UpdateOne(ds.Context, wst.M{"_id": id}, wst.M{"$set": *data}); err != nil {
			panic(err)
		}
		return findByObjectId(collectionName, id, ds, nil)
	}
	return nil, errors.New(fmt.Sprintf("invalid connector %v", connector))
}

func (ds *Datasource) DeleteById(collectionName string, id interface{}) int64 {
	var connector = ds.Viper.GetString(ds.Key + ".connector")
	switch connector {
	case "mongodb":
		var db = ds.Db.(*mongo.Client)

		database := db.Database(ds.Viper.GetString(ds.Key + ".database"))
		collection := database.Collection(collectionName)
		if result, err := collection.DeleteOne(ds.Context, wst.M{"_id": id}); err != nil {
			panic(err)
		} else {
			return result.DeletedCount
		}
	}
	return 0
}

func New(dsKey string, dsViper *viper.Viper, parentContext context.Context) *Datasource {
	name := dsViper.GetString(dsKey + ".name")
	if name == "" {
		name = dsKey
	}
	ctx, ctxCancelFn := context.WithCancel(parentContext)
	ds := &Datasource{
		Name:  name,
		Viper: dsViper,

		Key: dsKey,

		Context:     ctx,
		ctxCancelFn: ctxCancelFn,
	}
	return ds
}

func ReplaceObjectIds(data interface{}) interface{} {

	if data == nil {
		return nil
	}

	var finalData wst.M
	switch data.(type) {
	case int, int32, int64, float32, float64, bool, primitive.ObjectID, *primitive.ObjectID, time.Time, primitive.DateTime:
		return data
	case string:
		var newValue interface{}
		var err error
		if regexp.MustCompile("^([0-9a-f]{24})$").MatchString(data.(string)) {
			newValue, err = primitive.ObjectIDFromHex(data.(string))
			//} else if regexp.MustCompile("^(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:\\.\\d+)?)([+:\\-/0-9a-zA-Z]+)?$").MatchString(data.(string)) {
		} else if regexp.MustCompile("^(\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:\\.\\d+)?)([Z]+)?$").MatchString(data.(string)) {
			//	TODO: parse all type of dates
			//layout := "2006-01-02T15:04:05.000-03:00"
			layout := "2006-01-02T15:04:05.000Z"
			newValue, err = time.Parse(layout, data.(string))
		}
		if err != nil {
			log.Println("WARNING: ", err)
		}
		if newValue != nil {
			return newValue
		} else {
			return data
		}
	case wst.Where:
		finalData = wst.M{}
		for key, value := range data.(wst.Where) {
			finalData[key] = value
		}
		break
	case *wst.Where:
		finalData = wst.M{}
		for key, value := range *data.(*wst.Where) {
			finalData[key] = value
		}
		break
	case map[string]interface{}:
		finalData = wst.M{}
		for key, value := range data.(map[string]interface{}) {
			finalData[key] = value
		}
		break
	case *map[string]interface{}:
		finalData = wst.M{}
		for key, value := range *data.(*map[string]interface{}) {
			finalData[key] = value
		}
		break
	case wst.M:
		finalData = data.(wst.M)
		break
	case *wst.M:
		finalData = *data.(*wst.M)
		break
	default:
		log.Println(fmt.Sprintf("WARNING: Invalid input for ReplaceObjectIds() <- %s", data))
		return data
	}
	for key, value := range finalData {
		if value == nil {
			continue
		}
		var err error
		var newValue interface{}
		switch value.(type) {
		case string, wst.Where, *wst.Where, wst.M, *wst.M, int, int32, int64, float32, float64, bool, primitive.ObjectID, *primitive.ObjectID, time.Time, primitive.DateTime:
			newValue = ReplaceObjectIds(value)
			break
		default:
			asMap, asMapOk := value.(wst.M)
			if asMapOk {
				newValue = ReplaceObjectIds(asMap)
			} else {
				asList, asListOk := value.([]interface{})
				if asListOk {
					for i, asListItem := range asList {
						asList[i] = ReplaceObjectIds(asListItem)
					}
				} else {
					_, asStringListOk := value.([]string)
					if !asStringListOk {
						asMap, asMapOk := value.(map[string]interface{})
						if asMapOk {
							newValue = ReplaceObjectIds(asMap)
						} else {
							asList, asMListOk := value.([]wst.M)
							if asMListOk {
								for i, asListItem := range asList {
									asList[i] = ReplaceObjectIds(asListItem).(wst.M)
								}
							} else {
								asList, asGenericListOk := value.([]interface{})
								if asGenericListOk {
									for i, asListItem := range asList {
										asList[i] = ReplaceObjectIds(asListItem)
									}
								} else {
									_, asStringListOk := value.([]string)
									if !asStringListOk {
										log.Println(fmt.Sprintf("WARNING: What to do with %v (%s)?", value, value))
									}
								}
							}
						}
					}
				}
			}
		}
		if err == nil && newValue != nil {
			switch data.(type) {
			case wst.Where:
				data.(wst.Where)[key] = newValue
				break
			case *wst.Where:
				(*data.(*wst.Where))[key] = newValue
				break
			case wst.M:
				data.(wst.M)[key] = newValue
				break
			case *wst.M:
				(*data.(*wst.M))[key] = newValue
				break
			case map[string]interface{}:
				data.(map[string]interface{})[key] = newValue
				break
			case *map[string]interface{}:
				(*data.(*map[string]interface{}))[key] = newValue
				break
			default:
				log.Println(fmt.Sprintf("WARNING: invalid input ReplaceObjectIds() <- %s", data))
				break
			}
		} else if err != nil {
			log.Println("WARNING: ", err)
		}
	}
	return data
}
