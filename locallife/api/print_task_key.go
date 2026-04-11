package api

import (
	"fmt"
	"time"

	"github.com/merrydance/locallife/util"
)

func buildStableOrderPrintTaskKey(orderID int64, trigger string) string {
	return fmt.Sprintf("order:%d:%s", orderID, trigger)
}

func buildUniqueOrderPrintTaskKey(orderID int64, trigger string) string {
	return fmt.Sprintf("order:%d:%s:%d:%s", orderID, trigger, time.Now().UnixNano(), util.RandomString(6))
}

func buildRetryOrderPrintTaskKey(orderID int64, printLogID int64) string {
	return fmt.Sprintf("retry:%d:%d:%d:%s", orderID, printLogID, time.Now().UnixNano(), util.RandomString(6))
}
