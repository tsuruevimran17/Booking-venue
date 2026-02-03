package kafka

import "testing"

func TestSplitBrokers(t *testing.T) {
	input := "broker1:9092, broker2:9092,,"
	brokers := splitBrokers(input)

	if len(brokers) != 2 {
		t.Fatalf("expected 2 brokers, got %d", len(brokers))
	}
	if brokers[0] != "broker1:9092" || brokers[1] != "broker2:9092" {
		t.Fatalf("unexpected brokers slice: %#v", brokers)
	}
}
