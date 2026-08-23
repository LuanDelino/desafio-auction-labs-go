package auction

import (
	"context"
	"fmt"
	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/internal_error"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuctionEntityMongo struct {
	Id          string                          `bson:"_id"`
	ProductName string                          `bson:"product_name"`
	Category    string                          `bson:"category"`
	Description string                          `bson:"description"`
	Condition   auction_entity.ProductCondition `bson:"condition"`
	Status      auction_entity.AuctionStatus    `bson:"status"`
	Timestamp   int64                           `bson:"timestamp"`
}
type AuctionRepository struct {
	Collection *mongo.Collection
}

func NewAuctionRepository(database *mongo.Database) *AuctionRepository {
	return &AuctionRepository{
		Collection: database.Collection("auctions"),
	}
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   auctionEntity.Condition,
		Status:      auctionEntity.Status,
		Timestamp:   auctionEntity.Timestamp.Unix(),
	}
	_, err := ar.Collection.InsertOne(ctx, auctionEntityMongo)
	if err != nil {
		logger.Error("Error trying to insert auction", err)
		return internal_error.NewInternalServerError("Error trying to insert auction")
	}

	auctionId := auctionEntity.Id
	startAuctionCloseTimer(getAuctionInterval(), func() {
		// Contexto proprio: o ctx da requisicao que criou o leilao ja foi
		// cancelado quando o prazo vence, e o update morreria com ele.
		closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := ar.closeAuction(closeCtx, auctionId); err != nil {
			logger.Error(fmt.Sprintf("Error trying to close auction %s", auctionId), err)
		}
	})

	return nil
}

// closeAuction marca o leilao como Completed. O filtro por status Active deixa a
// operacao idempotente: fechar duas vezes nao sobrescreve nada.
func (ar *AuctionRepository) closeAuction(
	ctx context.Context, auctionId string) *internal_error.InternalError {
	filter := bson.M{"_id": auctionId, "status": auction_entity.Active}
	update := bson.M{"$set": bson.M{"status": auction_entity.Completed}}

	if _, err := ar.Collection.UpdateOne(ctx, filter, update); err != nil {
		logger.Error(fmt.Sprintf("Error trying to close auction %s", auctionId), err)
		return internal_error.NewInternalServerError("Error trying to close auction")
	}

	return nil
}

// getAuctionInterval devolve a duracao do leilao definida em AUCTION_INTERVAL,
// a mesma variavel que o repositorio de lances usa para recusar lance vencido.
func getAuctionInterval() time.Duration {
	auctionInterval := os.Getenv("AUCTION_INTERVAL")
	duration, err := time.ParseDuration(auctionInterval)
	if err != nil {
		return time.Minute * 5
	}

	return duration
}

// startAuctionCloseTimer aguarda a duracao em segundo plano e executa a acao de
// fechamento, sem bloquear a rotina que criou o leilao.
func startAuctionCloseTimer(duration time.Duration, closeAction func()) {
	go func() {
		timer := time.NewTimer(duration)
		defer timer.Stop()

		<-timer.C
		closeAction()
	}()
}
