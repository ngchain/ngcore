package ngtypes

import (
	"fmt"
	"math/big"
)

var GenesisBalances map[string][]byte

func init() {
	GenesisBalances = make(map[string][]byte)
	strMap := map[string]string{
		"JPGXLLFR3eUYgRnctAVwB2m4RQDgQokFLjyA4QUvRQ3UGLiE": "19995800000000000000",
		"N4RZBB6JLwJpPbCPtjy9eKqJe7UoFGCUVrz6YGS6XGn51ApT": "470354102952000000000",
		"Lo4odLdP1MWpmvq7yKwJ5cLEeRqsNf8d2i5QVwWRNUYV92Ua": "167393260344363631007837",
		"LcNcrpsGZ7XxPUNfw4Wj2YPoENtot57Jo2G2zRXFzzKHFPAW": "1102308224808000000000",
		"Et73BZBgFWUZiFrHsgeMjRoWYGzahQYAgAy4t5xztLwUmfvA": "37800000000000000",
		"RwMhTnFNj84sSTkiuW9pjD2zoBLGG5RgydcvoXf196MuTVrX": "1997397999999999970",
		"BeaDaq2xVa697RGBu5hZ48EYMdgbNVd1kr9DgZoTFsymY4Y":  "6335115345155000000000",
		"9sS1cdn8GPLG6NfD73FwYTUH6pK7tvaWMwmd7Xd2WA5nsC27": "185472575497271319402752",
		"KZP5ugr3dqdhPMXrYjBtG2JYZ2w8XS5sJsHZnRUXdWP6HEBo": "52830633055838000000000",
		"KX9Grs4iVtL4k9mfMxqUny8UMP2dyYbNuw61JYsHrqE9MbCK": "1254446670000000000000",
		"RDnTLnnRjj3PucfBzXq8HSoNrC4v7ibMkhv2yXcpR38LVrUh": "16937085075848000000000",
		"NyunbzpfjjB9QpuX7ZuZYvwKEQKEX7fu8EVLyREwq2AS7WT1": "219721260000000000000",
		"PVtPy2uWKgD3b2J9wzwMtbeBTxqFQsu1cdQTDYQGnA6uKqo6": "141938196162509269062710",
		"3a3ZjpWqX8XsNyW7bRSwPDHhzAY3rEvLeBqwMTSYjTFuWt3e": "12345543210000000000000",
		"DfGf8ZtxYuaNuyK4M4UXzZzWMDg34Ze6JMWPcqNAvqDST2N":  "362433999460601809710888",
		"H4p9rDMjKZtDkiedbG3EcdReExN1Fs5RSSuwbXmAY24d8D5W": "365915661211488927357164",
		"coutNLcFDZ283CxwtQHkFtF5ngAptjEkUSkVVaS6kFT9QCj":  "558281168483068014434913",
		"79SEtu9JcBSTgNoFRY5LDaTS34SE18sdVhTyh3NQmVkUF2mu": "669133425468142987019504",
		"GLmNguG25JtqjWEhYwJaTWxtByJDkmYnAEGgSzZTc7FtGKpe": "55483269452432000000000",
		"EYmzQYAN6veU2jGQFU6oWQBCkS5KaC3ht3eDhGQjUEvW2ndB": "179053653521844647584660",
		"PS2BkqouhzXG7iUaoDi3TsLi6z18yLHeUirtxm8mdqCeAg9Q": "20406488335845485912235",
		"BAPrS65YAbgK6Sb6vm3BZytramoZSnTkVQeh7GZVkKpG3RMR": "2083814573947194389443",
		"Qwthbiv6pgLNqkYMbU7cyJiDq4g7vE6VFv2oJ1uaokihXRPA": "10362363463272000000000",
		"CQiHNh5ip3K8YJpS84PeUFkrwTgkiePsExN4M35Pmu5nj3mG": "19404358913906000000000",
		"GQ3VLqbkyivMH1HJ1btxDU41EH8cKjSsjpa4NEuznXTPCcfV": "160407000000000000000000",
		"CFeWRU6AJKRKe2GXsP3rKMkypqjvPspEFrZHFDAy5i8bmKo7": "3213100329527000000000",
		"9W3eVBRzjHyhbszCZYnbapCdvVL9tfQKBr9w9RzvLaNx3vYg": "17779873888540000000000",
		"HAAti9NDKvZBrSiGH97etV79Lu8YAyKVTKRW9k7x2dWNkp5Q": "91710735383000000000",
		"Gz8jWRZCebFpE2GfQMjbWQ8L5nxGqDom2snia9f5KzPD4vBF": "358642632874769478059978",
		"FCM9yMw8VibFUXLKPQGJCmLpHQbZ6tqTiyeaHsWKAdoWuzFU": "9702218059836000000000",
		"5sbxYdYMEiD1rMpxDYVpdccJ8WSar6qJieBoFwi4FbeG5VJ6": "3277490944134000000000",
		"H7xQF64AJ5hSGcYWpEz57dEWb1qfNkVkworpLN1AwfL6nWqT": "7048600540936855739981830",
	}

	for strAddr, strBal := range strMap {
		bal, ok := new(big.Int).SetString(strBal, 10)
		if !ok {
			panic(fmt.Errorf("failed to load balance: %s", strBal))
		}

		GenesisBalances[strAddr] = bal.Bytes()
	}
}