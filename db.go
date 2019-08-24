package londo

import (
	"context"
	"errors"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	context context.Context
	client  *mongo.Client
	Name    string
}

func NewDBConnection(c *Config) (*MongoDB, error) {
	m := &MongoDB{Name: c.DB.Name}

	ctx := context.Background()
	m.context = ctx

	client, err := mongo.Connect(m.context, options.Client().ApplyURI(
		"mongodb://"+c.DB.Hostname+":"+strconv.Itoa(c.DB.Port)))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(m.context, nil); err != nil {
		return m, err
	}

	m.client = client
	return m, nil
}

func (m MongoDB) Disconnect() error {
	return m.client.Disconnect(m.context)
}

func (m MongoDB) FindAllSubjects() ([]*Subject, error) {
	col := m.client.Database(m.Name).Collection("subjects")

	cur, err := col.Find(m.context, m.client)
	if err != nil {
		return nil, err
	}
	defer cur.Close(m.context)

	var results []*Subject

	for cur.Next(m.context) {
		var res Subject

		err := cur.Decode(&res)
		if err != nil {
			return nil, err
		}
		results = append(results, &res)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (m MongoDB) FindExpiringSubjects(hours int) ([]*Subject, error) {
	subjs, err := m.FindAllSubjects()
	if err != nil {
		return nil, err
	}

	var res []*Subject

	for _, s := range subjs {
		h := time.Until(s.NotAfter).Round(time.Hour).Hours()
		if h < float64(hours) {
			res = append(res, s)
		}
	}

	return res, nil
}

func (m MongoDB) InsertSubject(s *Subject) error {
	col := m.getSubjCollection()
	s.ID = primitive.NewObjectID()

	_, err := col.InsertOne(m.context, s)
	return err
}

func (m MongoDB) DeleteSubject(hexId string, certid int) error {
	col := m.getSubjCollection()

	id, err := primitive.ObjectIDFromHex(hexId)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": id, "cert_id": certid}

	dres, err := col.DeleteOne(m.context, filter)
	if err != nil {
		return err
	}

	if dres.DeletedCount == 0 {
		return errors.New("no certificate with id " + strconv.Itoa(certid) + " found")
	}
	return err
}

func (m MongoDB) UpdateSubjCert(certId int, cert string, na time.Time) error {
	col := m.getSubjCollection()

	filter := bson.M{"cert_id": certId}
	update := bson.D{
		{"$set", bson.D{
			{"certificate", cert},
			{"not_after", na},
			{"updated_at", time.Now()},
		}},
	}

	_, err := col.UpdateOne(m.context, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (m MongoDB) FindSubject(s string) (Subject, error) {
	col := m.getSubjCollection()
	filter := bson.M{"subject": s}
	var res Subject

	err := col.FindOne(m.context, filter).Decode(&res)
	return res, err
}

func (m MongoDB) getSubjCollection() *mongo.Collection {
	return m.client.Database(m.Name).Collection("subjects")
}

type Subject struct {
	ID          primitive.ObjectID `bson:"_id"`
	Subject     string             `bson:"subject"`
	CSR         string             `bson:"csr"`
	PrivateKey  string             `bson:"private_key"`
	Certificate string             `bson:"certificate"`
	CertID      int                `bson:"cert_id"`
	OrderID     string             `bson:"order_id"`
	NotAfter    time.Time          `bson:"not_after"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	Targets     []string           `bson:"targets"`
	AltNames    []string           `bson:"alt_names"`
}
