package londo

import (
	"context"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Ctx    context.Context
	Client *mongo.Client
	Name   string
}

func NewDBConnection(c *Config) (*MongoDB, error) {
	m := &MongoDB{Name: c.DB.Name}

	ctx := context.Background()
	m.Ctx = ctx

	client, err := mongo.Connect(m.Ctx, options.Client().ApplyURI(
		"mongodb://"+c.DB.Hostname+":"+strconv.Itoa(c.DB.Port)))
	if err != nil {
		return nil, err
	}

	m.Client = client
	return m, nil
}

func (m MongoDB) Disconnect() {
	m.Client.Disconnect(m.Ctx)
}

func (m MongoDB) FindAllSubjects() ([]*Subject, error) {
	col := m.Client.Database(m.Name).Collection("subjects")

	cur, err := col.Find(m.Ctx, m.Client)
	if err != nil {
		return nil, err
	}
	defer cur.Close(m.Ctx)

	var results []*Subject

	for cur.Next(m.Ctx) {
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

type Subject struct {
	Subject     string    `bson:"subject"`
	CSR         string    `bson:"csr"`
	PrivateKey  string    `bson:"private_key"`
	Certificate string    `bson:"certificate"`
	CertID      int       `bson:"cert_id"`
	OrderID     string    `bson:"order_id"`
	NotAfter    time.Time `bson:"not_after"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
	Targets     []string  `bson:"targets"`
}
