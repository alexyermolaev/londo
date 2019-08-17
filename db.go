package londo

import (
	"context"
	"strconv"
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

func NewDBConnection(c *Config) (*MongoDB, error) {
	m := &MongoDB{Name: c.DB.Name}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	m.CancelCtx = cancel
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

func (m MongoDB) FindAll(c Collection) ([]*Collection, error) {
	col := m.Client.Database(m.Name).Collection(c.GetCollectioName())

	cur, err := col.Find(m.Ctx, m.Client)
	if err != nil {
		return nil, err
	}
	defer cur.Close(m.Ctx)

	var results []*Collection

	for cur.Next(m.Ctx) {
		var res Collection

		err := cur.Decode(res)
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

type Collection interface {
	GetCollectioName() string
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

func (s Subject) GetCollectionName() string {
	return "subjects"
}

func (s Subject) GetNotAfterHours() (float64, error) {
	pub, err := ParsePublicCertificate(s)
	if err != nil {
		return 0, err
	}

	diff := time.Until(pub.NotAfter)
	return diff.Round(time.Hour).Hours(), nil
}

func (s Subject) IsExpiring(hours int) (bool, float64, error) {
	h, err := s.GetNotAfterHours()
	if err != nil {
		return false, h, err
	}
	if h < float64(hours) {
		return true, h, err
	} else {
		return false, h, err
	}
}
