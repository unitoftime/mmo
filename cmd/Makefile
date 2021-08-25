lag-international:
	tc qdisc add dev lo root netem delay 200ms 40ms 25% loss 15.3% 25% duplicate 1% corrupt 0.1% reorder 5% 50%
lag-local:
	tc qdisc add dev lo root netem delay 25ms 5ms 25% loss 1% 5% duplicate 1% corrupt 0.1% reorder 1% 50%
lag-good:
	tc qdisc add dev lo root netem delay 25ms 5ms 25%
lag-remove:
	tc qdisc del dev lo root netem
