package matching

import (
	"context"
	"time"

	"github.com/nite-coder/blackbear/pkg/cast"
	"github.com/shopspring/decimal"
)

type Side int8

const (
	Buy  Side = 1
	Sell Side = 2
)

type OrderType string

const (
	Market   OrderType = "market"
	Limit    OrderType = "limit"
	FOK      OrderType = "fok"       // 全部成交或立即取消
	IOC      OrderType = "ioc"       // 立即成交并取消剩余
	PostOnly OrderType = "post_only" // be maker order only
	Cancel   OrderType = "cancel"    // the order has been canceled
)

type Order struct {
	ID        string          `json:"id"`
	MarketID  string          `json:"market_id"`
	Side      Side            `json:"side"`
	Price     decimal.Decimal `json:"price"`
	Size      decimal.Decimal `json:"size"`
	Type      OrderType       `json:"type"`
	UserID    int64           `json:"user_id"`
	CreatedAt time.Time       `json:"created_at"`
}

type Trade struct {
	ID             string          `json:"id"`
	MarketID       string          `json:"market_id"`
	TakerOrderID   string          `json:"taker_order_id"`
	TakerOrderSide Side            `json:"taker_order_side"`
	TakerOrderType OrderType       `json:"taker_order_type"`
	TakerUserID    int64           `json:"taker_user_id"`
	MakerOrderID   string          `json:"maker_order_id"`
	MakerUserID    int64           `json:"maker_user_id"`
	Price          decimal.Decimal `json:"price"`
	Size           decimal.Decimal `json:"size"`
	IsCancel       bool            `json:"is_cancel"`
	CreatedAt      time.Time       `json:"created_at"`
}

type Response struct {
	Error error
	Data  any
}

type Message struct {
	Action  string
	Payload any
	Resp    chan *Response
}

type OrderBookUpdateEvent struct {
	Bids []*UpdateEvent
	Asks []*UpdateEvent
	Time time.Time
}

type Depth struct {
	Asks []*DepthItem
	Bids []*DepthItem
}

// OrderBook type
type OrderBook struct {
	bidQueue      *queue
	askQueue      *queue
	orderChan     chan *Order
	cancelChan    chan string
	depthChan     chan *Message
	publishTrader PublishTrader
}

func NewOrderBook(publishTrader PublishTrader) *OrderBook {
	return &OrderBook{
		bidQueue:      NewBuyerQueue(),
		askQueue:      NewSellerQueue(),
		orderChan:     make(chan *Order, 1000000),
		cancelChan:    make(chan string, 1000000),
		depthChan:     make(chan *Message, 1000000),
		publishTrader: publishTrader,
	}
}

func (book *OrderBook) AddOrder(ctx context.Context, order *Order) error {
	if len(order.Type) == 0 || len(order.ID) == 0 {
		return ErrInvalidParam
	}

	select {
	case book.orderChan <- order:
		return nil
	case <-ctx.Done():
		return ErrTimeout
	}
}

func (book *OrderBook) CancelOrder(ctx context.Context, id string) error {
	if len(id) == 0 {
		return nil
	}

	select {
	case book.cancelChan <- id:
		return nil
	case <-ctx.Done():
		return ErrTimeout
	}
}

func (book *OrderBook) Depth(limit uint32) (*Depth, error) {
	if limit == 0 {
		return nil, ErrInvalidParam
	}

	msg := &Message{
		Action:  "depth",
		Payload: limit,
		Resp:    make(chan *Response),
	}

	select {
	case book.depthChan <- msg:
		resp := <-msg.Resp
		if resp.Error != nil {
			return nil, resp.Error
		}

		if resp.Data != nil {
			result, ok := resp.Data.(*Depth)
			if ok {
				return result, nil
			}
		}

		return nil, nil
	case <-time.After(time.Second):
		return nil, ErrTimeout
	}
}

func (book *OrderBook) Start() error {
	for {
		select {
		case order := <-book.orderChan:
			book.addOrder(order)
		case orderID := <-book.cancelChan:
			book.cancelOrder(orderID)
		case msg := <-book.depthChan:
			limit, _ := cast.ToUint32(msg.Payload)
			result := book.depth(limit)
			resp := Response{
				Error: nil,
				Data:  result,
			}

			msg.Resp <- &resp
		}
	}
}

func (book *OrderBook) addOrder(order *Order) {
	var trades []*Trade

	switch order.Type {
	case Limit, FOK, IOC, PostOnly, Cancel:
		trades, _ = book.handleOrder(order)
	case Market:
		trades, _ = book.handleMarketOrder(order)
	}

	if len(trades) > 0 {
		book.publishTrader.PublishTrades(trades...)
	}
}

func (book *OrderBook) cancelOrder(id string) {
	order := book.askQueue.order(id)
	if order != nil {
		book.askQueue.removeOrder(order.Price, id)
		return
	}

	order = book.bidQueue.order(id)
	if order != nil {
		book.bidQueue.removeOrder(order.Price, id)
		return
	}
}

func (book *OrderBook) depth(limit uint32) *Depth {
	return &Depth{
		Asks: book.askQueue.depth(limit),
		Bids: book.bidQueue.depth(limit),
	}
}

func (book *OrderBook) handleOrder(order *Order) ([]*Trade, error) {
	var myQueue, targetQueue *queue
	if order.Side == Buy {
		myQueue = book.bidQueue
		targetQueue = book.askQueue
	} else {
		myQueue = book.askQueue
		targetQueue = book.bidQueue
	}

	trades := []*Trade{}

	// ensure the order book can handle FOK order
	if order.Type == FOK {

		el := targetQueue.depthList.Front()
		orignalOrderSize := order.Size

		for {
			if el == nil {
				trade := Trade{
					MarketID:       order.MarketID,
					TakerOrderID:   order.ID,
					TakerOrderSide: order.Side,
					TakerOrderType: order.Type,
					TakerUserID:    order.UserID,
					MakerOrderID:   order.ID,
					MakerUserID:    order.UserID,
					Price:          order.Price,
					Size:           order.Size,
					IsCancel:       true,
					CreatedAt:      time.Now().UTC(),
				}
				trades = append(trades, &trade)
				return trades, nil
			}

			unit, _ := el.Value.(*priceUnit)
			tOrd, _ := unit.list.Front().Value.(*Order)

			if order.Side == Buy && order.Price.GreaterThanOrEqual(tOrd.Price) && order.Size.GreaterThanOrEqual(unit.totalSize) ||
				order.Side == Sell && order.Price.LessThanOrEqual(tOrd.Price) && order.Size.GreaterThanOrEqual(unit.totalSize) {
				order.Size = order.Size.Sub(tOrd.Size)

				if order.Size.Equal(decimal.Zero) {
					order.Size = orignalOrderSize
					break
				}
			}

			if order.Size.LessThan(decimal.Zero) {
				trade := Trade{
					MarketID:       order.MarketID,
					TakerOrderID:   order.ID,
					TakerOrderSide: order.Side,
					TakerOrderType: order.Type,
					TakerUserID:    order.UserID,
					MakerOrderID:   order.ID,
					MakerUserID:    order.UserID,
					Price:          order.Price,
					Size:           order.Size,
					IsCancel:       true,
					CreatedAt:      time.Now().UTC(),
				}
				trades = append(trades, &trade)
				return trades, nil
			}

			el = el.Next()
		}
	}

	for {
		tOrd := targetQueue.popHeadOrder()

		if tOrd == nil {
			switch order.Type {
			case Limit, PostOnly:
				myQueue.insertOrder(order, false)
				return trades, nil
			case IOC:
				trade := Trade{
					MarketID:       order.MarketID,
					TakerOrderID:   order.ID,
					TakerOrderSide: order.Side,
					TakerOrderType: order.Type,
					TakerUserID:    order.UserID,
					MakerOrderID:   order.ID,
					MakerUserID:    order.UserID,
					Price:          order.Price,
					Size:           order.Size,
					IsCancel:       true,
					CreatedAt:      time.Now().UTC(),
				}
				trades = append(trades, &trade)
				return trades, nil
			}
		}

		if order.Side == Buy && order.Price.LessThan(tOrd.Price) ||
			order.Side == Sell && order.Price.GreaterThan(tOrd.Price) {
			targetQueue.insertOrder(tOrd, true)

			switch order.Type {
			case Limit, PostOnly:
				myQueue.insertOrder(order, false)
				return trades, nil
			case IOC:
				trade := Trade{
					MarketID:       order.MarketID,
					TakerOrderID:   order.ID,
					TakerOrderSide: order.Side,
					TakerOrderType: order.Type,
					TakerUserID:    order.UserID,
					MakerOrderID:   order.ID,
					MakerUserID:    order.UserID,
					Price:          order.Price,
					Size:           order.Size,
					IsCancel:       true,
					CreatedAt:      time.Now().UTC(),
				}
				trades = append(trades, &trade)
				return trades, nil
			}
		}

		if order.Type == PostOnly {
			targetQueue.addOrder(tOrd)
			trade := Trade{
				MarketID:       order.MarketID,
				TakerOrderID:   order.ID,
				TakerOrderSide: order.Side,
				TakerOrderType: order.Type,
				TakerUserID:    order.UserID,
				MakerOrderID:   order.ID,
				MakerUserID:    order.UserID,
				Price:          order.Price,
				Size:           order.Size,
				IsCancel:       true,
				CreatedAt:      time.Now().UTC(),
			}
			trades = append(trades, &trade)
			return trades, nil
		}

		if order.Size.GreaterThanOrEqual(tOrd.Size) {
			trade := Trade{
				TakerOrderID: order.ID,
				MakerOrderID: tOrd.ID,
				Price:        tOrd.Price,
				Size:         tOrd.Size,
				CreatedAt:    time.Now().UTC(),
			}
			trades = append(trades, &trade)
			order.Size = order.Size.Sub(tOrd.Size)

			if order.Size.Equal(decimal.Zero) {
				break
			}
		} else {
			trade := Trade{
				TakerOrderID: order.ID,
				MakerOrderID: tOrd.ID,
				Price:        tOrd.Price,
				Size:         order.Size,
				CreatedAt:    time.Now().UTC(),
			}
			trades = append(trades, &trade)
			tOrd.Size = tOrd.Size.Sub(order.Size)
			targetQueue.insertOrder(tOrd, true)

			break
		}
	}

	return trades, nil
}

func (book *OrderBook) handleMarketOrder(order *Order) ([]*Trade, error) {
	targetQueue := book.bidQueue
	if order.Side == Buy {
		targetQueue = book.askQueue
	}

	trades := []*Trade{}

	for {
		tOrd := targetQueue.popHeadOrder()

		if tOrd == nil {
			trade := Trade{
				MarketID:       order.MarketID,
				TakerOrderID:   order.ID,
				TakerOrderSide: order.Side,
				TakerOrderType: order.Type,
				TakerUserID:    order.UserID,
				MakerOrderID:   order.ID,
				MakerUserID:    order.UserID,
				Price:          order.Price,
				Size:           order.Size,
				IsCancel:       true,
				CreatedAt:      time.Now().UTC(),
			}
			trades = append(trades, &trade)
			return trades, nil
		}

		// The size of the market order is the total amount, not the quantity.
		amount := tOrd.Price.Mul(tOrd.Size)

		if order.Size.GreaterThanOrEqual(amount) {
			trade := Trade{
				TakerOrderID: order.ID,
				MakerOrderID: tOrd.ID,
				Price:        tOrd.Price,
				Size:         tOrd.Size,
				CreatedAt:    time.Now().UTC(),
			}
			trades = append(trades, &trade)
			order.Size = order.Size.Sub(amount)
			if order.Size.Equal(decimal.Zero) {
				break
			}
		} else {
			tSize := order.Size.Div(tOrd.Price)

			trade := Trade{
				TakerOrderID: order.ID,
				MakerOrderID: tOrd.ID,
				Price:        tOrd.Price,
				Size:         tSize,
				CreatedAt:    time.Now().UTC(),
			}
			trades = append(trades, &trade)

			tOrd.Size = tOrd.Size.Sub(tSize)
			targetQueue.insertOrder(tOrd, true)

			break
		}
	}

	return trades, nil
}
