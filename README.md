# Leilão com fechamento automático

API de leilões em Go. O projeto base (criação de leilão, lances e consultas) vem do
curso Go Expert; a feature adicionada é o encerramento automático do leilão quando sua
duração expira, sem intervenção manual.

## O que foi implementado

Tudo em `internal/infra/database/auction/create_auction.go`:

- `getAuctionInterval()` — duração do leilão a partir de `AUCTION_INTERVAL`, com padrão de 5 minutos.
- `startAuctionCloseTimer()` — goroutine que aguarda o prazo sem bloquear a resposta HTTP.
- `closeAuction()` — atualiza o status para `Completed`, filtrando por `Active` para ser idempotente.

A goroutine de fechamento usa contexto próprio: o `ctx` que chega em `CreateAuction` é
o da requisição HTTP e já foi cancelado quando o prazo vence, então o `update` morreria
com ele.

## Como rodar

```sh
docker compose up -d --build
```

Sobem três serviços: `mongodb`, `app` (que espera o banco ficar saudável) e `seed`, um
one-shot que insere usuários de teste e encerra. API em `http://localhost:8080`.

```sh
docker compose logs -f app     # acompanhar
docker compose down -v         # derrubar, apagando os dados
```

O `seed` roda a cada `compose up` e é idempotente. Para rodar por fora, ou com outros
valores, edite `seed.js` e execute:

```sh
docker compose up seed
```

Ele cria `8b3f6f1a-1c2d-4e5f-9a70-111111111111` (Ana Souza) e
`9c4a7e2b-2d3e-4f60-8b81-222222222222` (Bruno Lima). Sem eles, `GET /user/:userId` só
responde 404 — o projeto não tem endpoint que crie usuário.

### Exercitando o fechamento automático

Com `AUCTION_INTERVAL=20s`, o padrão do `.env`:

```sh
# cria o leilão
curl -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"product_name":"Monitor Ultrawide","category":"Perifericos","description":"Monitor ultrawide 34 polegadas em bom estado","condition":2}'

# pega o id na listagem
curl localhost:8080/auction

# manda 4 lances, que é o MAX_BATCH_SIZE: o lote vai para o banco na hora
curl -X POST localhost:8080/bid \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"8b3f6f1a-1c2d-4e5f-9a70-111111111111","auction_id":"<id>","amount":1200}'

# status 0 = Active; depois de 20s o mesmo leilão volta com status 1 = Completed
curl localhost:8080/auction/<id>

# e o lance vencedor é o maior do leilão fechado
curl localhost:8080/auction/winner/<id>
```

Com menos de 4 lances, o lote só vai para o banco quando passa `BATCH_INSERT_INTERVAL`
(20s) — um lance isolado perto do prazo pode ser descartado pelo próprio fechamento.

### Endpoints

| Método | Rota | Observação |
|--|--|--|
| POST | `/auction` | cria o leilão e agenda o fechamento |
| GET | `/auction` | lista todos; filtros opcionais `status`, `category`, `productName` |
| GET | `/auction/:auctionId` | 404 se não existir |
| GET | `/auction/winner/:auctionId` | leilão e maior lance |
| POST | `/bid` | 404 se o leilão não existir, 400 se estiver fechado |
| GET | `/bid/:auctionId` | lances do leilão |
| GET | `/user/:userId` | precisa do seed |

## Variáveis de ambiente

Em `cmd/auction/.env`, carregado pela aplicação e pelos serviços do compose.

| Variável | Padrão | Para que serve |
|--|--|--|
| `AUCTION_INTERVAL` | `20s` | duração do leilão: quanto tempo após a criação ele fecha sozinho |
| `BATCH_INSERT_INTERVAL` | `20s` | intervalo máximo até gravar o lote de lances |
| `MAX_BATCH_SIZE` | `4` | quantidade de lances que fecha o lote antes do intervalo |
| `MONGODB_URL` | `mongodb://admin:admin@mongodb:27017/auctions?authSource=admin` | conexão, usada também pelo seed |
| `MONGODB_DB` | `auctions` | banco da aplicação |

`AUCTION_INTERVAL` aceita qualquer duração no formato do Go (`30s`, `5m`, `1h30m`); se
faltar ou for inválida, a aplicação usa 5 minutos. Para ver o fechamento rápido:

```sh
sed -i 's/^AUCTION_INTERVAL=.*/AUCTION_INTERVAL=5s/' cmd/auction/.env
docker compose up -d --build
```

## Testes

```sh
# sem banco: unidade e rota
go test ./...

# com integração (precisa de um Mongo de verdade)
docker compose up -d mongodb
MONGODB_URL_TEST='mongodb://admin:admin@localhost:27017/auctions_test?authSource=admin' \
  go test -v -count=1 ./...
```

Sem `MONGODB_URL_TEST`, os testes de integração são ignorados (`skip`). Cada pacote usa
seu próprio banco (`auctions_test_auction`, `auctions_test_bid`), separado do banco da
aplicação, e limpa as coleções antes de cada caso — `go test ./...` roda os pacotes em
paralelo, então banco compartilhado entre pacotes daria teste instável.

| Teste | O que prova |
|--|--|
| `TestGetAuctionInterval` | lê a variável; cai no padrão de 5min se inválida ou ausente |
| `TestStartAuctionCloseTimer` | não bloqueia quem chamou, não dispara antes do prazo, dispara uma vez |
| `TestCloseAuctionIntegracao` | o fechamento escreve `Completed` no banco |
| `TestCreateAuctionFechaSozinhoIntegracao` | leilão nasce `Active` e fecha sozinho após o prazo |
| `TestCreateAuctionFechaMesmoComContextoDaRequisicaoCanceladoIntegracao` | o fechamento sobrevive ao fim da requisição HTTP |
| `TestFindAuctionsPorStatusIntegracao` | filtra abertos, fechados, e lista todos sem filtro |
| `TestFindAuctionByIdInexistenteIntegracao` | leilão inexistente devolve `not_found` |
| `TestFindAuctions` (rota) | `status` opcional, e `status` inválido segue recusado |
| `TestFindBidByAuctionIdIntegracao` | a listagem encontra os lances gravados |
| `TestCreateBid` | recusa lance em leilão inexistente, fechado, ou com `user_id` inválido |

Como a feature é concorrente, vale rodar com `-race` (exige `gcc`; sem ele, use o
container do Go):

```sh
docker run --rm -v "$PWD":/app -w /app \
  --network desafio-auction-goexpert_localNetwork \
  -e MONGODB_URL_TEST='mongodb://admin:admin@mongodb:27017/auctions_test?authSource=admin' \
  golang:1.24 go test -race -count=1 ./...
```

## Correções no projeto base

Fora do enunciado, mas necessárias para o conjunto funcionar:

- `GET /bid/:auctionId` sempre devolvia `null`: o filtro procurava `auctionId` e o
  documento grava `auction_id`.
- `GET /auction` respondia `400` sem o parâmetro `status`, e `?status=0` devolvia todos
  em vez de só os abertos. Agora `status` é opcional e `0` filtra `Active`.
- `GET /auction/:auctionId` com id inexistente respondia `500`; agora `404`, como o
  endpoint de usuário já fazia.
- `POST /bid` respondia `201` para leilão inexistente ou fechado e descartava o lance
  depois, sem erro e sem log. Agora recusa na hora, com `404` ou `400`.
- `fmt.Sprintf` com `%d` para o id do usuário, que fazia o vet embutido no `go test`
  derrubar o build do pacote.

## Limites conhecidos

- O agendamento vive no processo. Se a aplicação reiniciar antes do prazo, o leilão
  criado antes do restart permanece `Active` no banco — não há rotina de recuperação no
  boot. O repositório de lances recusa lance vencido por tempo, usando a mesma
  `AUCTION_INTERVAL` sobre o `timestamp` do leilão, então um leilão vencido não aceita
  lances mesmo com o status desatualizado.
- `AUCTION_INTERVAL` é lido em dois lugares (na feature e em
  `internal/infra/database/bid/create_bid.go`, que já existia), com a mesma chave e o
  mesmo padrão. Ler chaves diferentes abriria uma janela em que o leilão aparece aberto
  e descarta lances em silêncio.
- `POST /bid` não valida se o `user_id` existe, só o formato. O projeto não tem endpoint
  que crie usuário, então exigir usuário existente faria o seed virar dependência da
  escrita, não conveniência de teste.
