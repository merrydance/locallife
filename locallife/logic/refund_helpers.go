package logic

import (
	"crypto/rand"
	"fmt"
	"time"
)

func generateOutRefundNo() string {
	now := time.Now()
	dateStr := now.Format("20060102150405")

	b := make([]byte, 4)
	_, _ = rand.Read(b)
	randomNum := fmt.Sprintf("%08d", int(b[0])*1000000+int(b[1])*10000+int(b[2])*100+int(b[3]))

	return "R" + dateStr + randomNum[:8]
}
