package auction

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestGetAuctionInterval(t *testing.T) {
	t.Run("le a duracao da variavel de ambiente", func(t *testing.T) {
		t.Setenv("AUCTION_INTERVAL", "45s")

		if got := getAuctionInterval(); got != 45*time.Second {
			t.Errorf("esperava 45s, veio %s", got)
		}
	})

	t.Run("cai no padrao de 5 minutos quando a variavel esta invalida", func(t *testing.T) {
		t.Setenv("AUCTION_INTERVAL", "vinte segundos")

		if got := getAuctionInterval(); got != 5*time.Minute {
			t.Errorf("esperava 5m, veio %s", got)
		}
	})

	t.Run("cai no padrao de 5 minutos quando a variavel nao existe", func(t *testing.T) {
		t.Setenv("AUCTION_INTERVAL", "")

		if got := getAuctionInterval(); got != 5*time.Minute {
			t.Errorf("esperava 5m, veio %s", got)
		}
	})
}

func TestStartAuctionCloseTimer(t *testing.T) {
	t.Run("nao bloqueia quem chamou e nao dispara antes do prazo", func(t *testing.T) {
		var chamadas int32
		startAuctionCloseTimer(200*time.Millisecond, func() {
			atomic.AddInt32(&chamadas, 1)
		})

		if got := atomic.LoadInt32(&chamadas); got != 0 {
			t.Fatalf("acao disparou antes do prazo (%d chamadas), o agendamento bloqueou a rotina principal", got)
		}
	})

	t.Run("dispara a acao uma unica vez depois do prazo", func(t *testing.T) {
		disparou := make(chan struct{}, 2)
		startAuctionCloseTimer(100*time.Millisecond, func() {
			disparou <- struct{}{}
		})

		select {
		case <-disparou:
		case <-time.After(2 * time.Second):
			t.Fatal("a acao de fechamento nunca foi executada")
		}

		select {
		case <-disparou:
			t.Fatal("a acao de fechamento executou mais de uma vez")
		case <-time.After(300 * time.Millisecond):
		}
	})
}
