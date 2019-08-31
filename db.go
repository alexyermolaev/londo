package londo

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/streadway/amqp"
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

func (m *MongoDB) Disconnect() error {
	return m.client.Disconnect(m.context)
}

func (m *MongoDB) FindAllSubjects() ([]*Subject, error) {
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

func (m *MongoDB) FindExpiringSubjects(hours int) ([]*Subject, error) {
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

func (m *MongoDB) InsertSubject(s *Subject) error {
	col := m.getSubjCollection()
	s.ID = primitive.NewObjectID()

	_, err := col.InsertOne(m.context, s)
	return err
}

func (m *MongoDB) DeleteSubject(hexId string, certid int) error {
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

func (m *MongoDB) UpdateSubjCert(certId *int, cert *string, na *time.Time, sn *big.Int) error {
	col := m.getSubjCollection()

	filter := bson.M{"cert_id": certId}
	update := bson.D{
		{"$set", bson.D{
			{"certificate", cert},
			{"not_after", na},
			{"serial", sn.String()},
			{"updated_at", time.Now()},
			{"no_match", true},
		}},
	}

	_, err := col.UpdateOne(m.context, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (m *MongoDB) FindSubject(s string) (Subject, error) {
	col := m.getSubjCollection()
	filter := bson.M{"subject": s}
	var res Subject

	err := col.FindOne(m.context, filter).Decode(&res)
	return res, err
}

func (m *MongoDB) FineManySubjects(s []string) ([]Subject, error) {
	col := m.getSubjCollection()
	filter := bson.M{"targets": s}
	var res []Subject

	cur, err := col.Find(m.context, filter, nil)
	if err != nil {
		return nil, err
	}

	for cur.Next(m.context) {
		var s Subject
		if err := cur.Decode(&s); err != nil {
			return nil, err
		}
		res = append(res, s)
	}

	return res, nil
}

func (m *MongoDB) UpdateUnreachable(e *CheckCertEvent) error {
	col := m.getSubjCollection()

	filter := bson.M{"subject": e.Subject}
	update := bson.D{
		{"$set", bson.D{
			{"unresolvable_at", e.Unresolvable},
			{"match", e.Match},
			{"targets", e.Targets},
			{"outdated", e.Outdated},
			{"updated_at", time.Now()},
		}},
	}

	_, err := col.UpdateOne(m.context, filter, update)

	return err
}

func (m *MongoDB) getSubjCollection() *mongo.Collection {
	return m.client.Database(m.Name).Collection("subjects")
}

type Subject struct {
	ID             primitive.ObjectID `bson:"_id"`
	Subject        string             `bson:"subject"`
	Port           int32              `bson:"port"`
	CSR            string             `bson:"csr"`
	PrivateKey     string             `bson:"private_key"`
	Certificate    string             `bson:"certificate,omitempty"`
	Serial         string             `bson:"serial"`
	CertID         int                `bson:"cert_id"`
	OrderID        string             `bson:"order_id"`
	NotAfter       time.Time          `bson:"not_after"`
	CreatedAt      time.Time          `bson:"created_at"`
	UpdatedAt      time.Time          `bson:"updated_at"`
	UnresolvableAt time.Time          `bson:"unresolvable_at,omitempty"`
	Targets        []string           `bson:"targets,omitempty"`
	AltNames       []string           `bson:"alt_names,omitempty"`
	Match          bool               `bson:"match"`
	Outdated       []string           `bson:"outdated,omitempty"`
}

func (Subject) GetMessage() amqp.Publishing {
	return amqp.Publishing{}
}
