package auction_controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"fullcycle-auction_go/internal/internal_error"
	"fullcycle-auction_go/internal/usecase/auction_usecase"

	"github.com/gin-gonic/gin"
)

type auctionUseCaseDuble struct {
	statusRecebido auction_usecase.AuctionStatus
}

func (d *auctionUseCaseDuble) CreateAuction(
	ctx context.Context, auctionInput auction_usecase.AuctionInputDTO) *internal_error.InternalError {
	return nil
}

func (d *auctionUseCaseDuble) FindAuctionById(
	ctx context.Context, id string) (*auction_usecase.AuctionOutputDTO, *internal_error.InternalError) {
	return nil, nil
}

func (d *auctionUseCaseDuble) FindAuctions(
	ctx context.Context,
	status auction_usecase.AuctionStatus,
	category, productName string) ([]auction_usecase.AuctionOutputDTO, *internal_error.InternalError) {
	d.statusRecebido = status
	return []auction_usecase.AuctionOutputDTO{}, nil
}

func (d *auctionUseCaseDuble) FindWinningBidByAuctionId(
	ctx context.Context, auctionId string) (*auction_usecase.WinningInfoOutputDTO, *internal_error.InternalError) {
	return nil, nil
}

func chamarListagem(t *testing.T, query string) (*httptest.ResponseRecorder, *auctionUseCaseDuble) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	duble := &auctionUseCaseDuble{}
	router := gin.New()
	router.GET("/auction", NewAuctionController(duble).FindAuctions)

	resposta := httptest.NewRecorder()
	router.ServeHTTP(resposta, httptest.NewRequest(http.MethodGet, "/auction"+query, nil))

	return resposta, duble
}

func TestFindAuctions(t *testing.T) {
	t.Run("sem o parametro status lista todos", func(t *testing.T) {
		resposta, duble := chamarListagem(t, "")

		if resposta.Code != http.StatusOK {
			t.Fatalf("esperava 200, veio %d: %s", resposta.Code, resposta.Body.String())
		}
		if duble.statusRecebido != auction_usecase.NoStatusFilter {
			t.Errorf("esperava a listagem sem filtro, veio status %d", duble.statusRecebido)
		}
	})

	t.Run("status zero filtra os leiloes abertos", func(t *testing.T) {
		resposta, duble := chamarListagem(t, "?status=0")

		if resposta.Code != http.StatusOK {
			t.Fatalf("esperava 200, veio %d", resposta.Code)
		}
		if duble.statusRecebido != 0 {
			t.Errorf("esperava status 0 (Active), veio %d", duble.statusRecebido)
		}
	})

	t.Run("status invalido continua recusado", func(t *testing.T) {
		resposta, _ := chamarListagem(t, "?status=aberto")

		if resposta.Code != http.StatusBadRequest {
			t.Errorf("esperava 400, veio %d", resposta.Code)
		}
	})
}
