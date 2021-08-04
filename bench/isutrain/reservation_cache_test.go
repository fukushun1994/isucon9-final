package isutrain

import (
	"log"
	"testing"
	"time"

	"github.com/chibiegg/isucon9-final/bench/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestReservationMem_CanReserve_Kudari(t *testing.T) {
	now := time.Now()
	mem := newReservationCache()

	gotTests := []struct {
		reservationID int
		req           *ReserveRequest
	}{
		{
			reservationID: 10,
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "古岡",
				Arrival:    "荒川",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
		},
	}

	for _, gotTest := range gotTests {
		user := &User{
			Email:    "hoge@example.com",
			Password: "hoge",
		}

		mem.Add(user, gotTest.req, gotTest.reservationID)
	}

	wantTests := []struct {
		req        *ReserveRequest
		canReserve bool
		err        error
	}{
		// 日付
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(2 * time.Minute)),
				Departure:  "古岡",
				Arrival:    "荒川",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: true,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "古岡",
				Arrival:    "荒川",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		// 座席
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "古岡",
				Arrival:    "荒川",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    9999,
						Column: "column9999",
					},
				},
			},
			canReserve: true,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "古岡",
				Arrival:    "荒川",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		// 区間
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "東京",
				Arrival:    "磯川",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "山田",
				Arrival:    "鳴門",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "東京",
				Arrival:    "山田",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "東京",
				Arrival:    "古岡",
				TrainClass: "badtest1",
				TrainName:  "badtest1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "badc0lumn1",
					},
				},
			},
			canReserve: true,
			err:        nil,
		},
		{ // failed
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "荒川",
				Arrival:    "鳴門",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
					&TrainSeat{
						Row:    2,
						Column: "column2",
					},
				},
			},
			canReserve: true,
			err:        nil,
		},
	}

	for idx, wantTest := range wantTests {
		log.Printf("[test%d]\n", idx)
		canReserve, err := mem.CanReserve(wantTest.req)
		assert.Equal(t, wantTest.err, err)
		assert.Equal(t, wantTest.canReserve, canReserve, "test%d failed", idx)
		log.Println("=============")
	}
}

func TestReservationMem_CanReserve_Nobori(t *testing.T) {
	now := time.Now()
	mem := newReservationCache()

	gotTests := []struct {
		reservationID int
		req           *ReserveRequest
	}{
		{
			reservationID: 10,
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "荒川",
				Arrival:    "古岡",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
		},
	}

	for _, gotTest := range gotTests {
		user := &User{
			Email:    "fuga@example.com",
			Password: "fuga",
		}
		mem.Add(user, gotTest.req, gotTest.reservationID)
	}

	wantTests := []struct {
		req        *ReserveRequest
		canReserve bool
		err        error
	}{
		// 日付
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(2 * time.Minute)),
				Departure:  "荒川",
				Arrival:    "古岡",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: true,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "荒川",
				Arrival:    "古岡",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		// 座席
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "荒川",
				Arrival:    "古岡",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    9999,
						Column: "column9999",
					},
				},
			},
			canReserve: true,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "荒川",
				Arrival:    "古岡",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		// 区間
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "磯川",
				Arrival:    "東京",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "鳴門",
				Arrival:    "山田",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "山田",
				Arrival:    "東京",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
				},
			},
			canReserve: false,
			err:        nil,
		},
		{
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "古岡",
				Arrival:    "東京",
				TrainClass: "badtest1",
				TrainName:  "badtest1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "badc0lumn1",
					},
				},
			},
			canReserve: true,
			err:        nil,
		},
		{ // failed
			req: &ReserveRequest{
				Date:       util.FormatISO8601(now.Add(time.Minute)),
				Departure:  "鳴門",
				Arrival:    "荒川",
				TrainClass: "test1",
				TrainName:  "test1",
				CarNum:     1,
				Seats: TrainSeats{
					&TrainSeat{
						Row:    1,
						Column: "column1",
					},
					&TrainSeat{
						Row:    2,
						Column: "column2",
					},
				},
			},
			canReserve: true,
			err:        nil,
		},
	}

	for idx, wantTest := range wantTests {
		log.Printf("[test%d]\n", idx)
		canReserve, err := mem.CanReserve(wantTest.req)
		assert.Equal(t, wantTest.err, err)
		assert.Equal(t, wantTest.canReserve, canReserve, "test%d failed", idx)
		log.Println("=============")
	}
}
