package groupux

import (
	"strings"
	"time"
)

type DigestRenderItem struct {
	Index   int
	Time    string
	Site    string
	Type    string
	UID     string
	Preview string
}

func BuildDigestRenderItems(items []QuietSummaryItem, maxItems int, loc *time.Location) ([]DigestRenderItem, int) {
	if maxItems <= 0 {
		maxItems = 1
	}
	if loc == nil {
		loc = time.Local
	}
	limit := len(items)
	if limit > maxItems {
		limit = maxItems
	}
	rendered := make([]DigestRenderItem, 0, limit)
	for i := 0; i < limit; i++ {
		item := items[i]
		occur := item.OccurredAt
		if occur.IsZero() {
			occur = time.Now()
		}
		rendered = append(rendered, DigestRenderItem{
			Index:   i + 1,
			Time:    occur.In(loc).Format("15:04:05"),
			Site:    item.Site,
			Type:    item.Type.String(),
			UID:     item.UID,
			Preview: CompactMessagePreview(item.Message, 120),
		})
	}
	return rendered, len(items) - limit
}

func CompactMessagePreview(input string, max int) string {
	if max <= 0 {
		max = 120
	}
	replacer := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ")
	line := strings.TrimSpace(replacer.Replace(input))
	line = strings.Join(strings.Fields(line), " ")
	if line == "" {
		return "(\u7a7a\u6d88\u606f)"
	}
	runes := []rune(line)
	if len(runes) > max {
		return string(runes[:max]) + "..."
	}
	return line
}
