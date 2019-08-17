package londo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
	Ctx       context.Context
	CancelCtx context.CancelFunc
	Client    *mongo.Client
	Name      string
}

func NewDBConnection(db string) (*MongoDB, error) {
	m := &MongoDB{Name: db}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.CancelCtx = cancel
	m.Ctx = ctx

	client, err := mongo.Connect(m.Ctx, options.Client().ApplyURI("mongodb://"+"localhost"+":"+"27017"))
	if err != nil {
		return nil, err
	}

	m.Client = client
	return m, nil
}

type Subject struct {
	Subject     string    `bson:"subject"`
	CSR         string    `bson:"csr"`
	PrivateKey  string    `bson:"private_key"`
	Certificate string    `bson:"certificate"`
	CertID      int       `bson:"cert_id"`
	OrderID     string    `bson:"order_id"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
	Targets     []string  `bson:"targets"`
}

func (s Subject) GetCollectionName(db string, c *mongo.Client) *mongo.Collection {
	return c.Database(db).Collection("subjects")
}
