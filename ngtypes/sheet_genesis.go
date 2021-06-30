package ngtypes

import (
	"fmt"
	"github.com/mr-tron/base58"
	"math/big"
)

var genesisBalances []*Balance
var genesisSheet *Sheet

func init() {
	strMap := map[string]string{
		"23gC8K2FTR9FreVkkUTNYptg5S7i9VH7aa9zQLYyiwXDkPbG": "184000000000000000000",
		"2bM9VjWGp5sfXxD14St39WjqscNqmc4Hu8UP4VJRHfkMAqR4": "360000000000000000000",
		"2tFu3NBw14ovDkGkf7rtpJ22zTmdu6crRhKeYgCuFfQvrkMf": "10000000000000000000",
		"3Kd3DdZiKXuexCahMNUfm7QeVi6affaVd7465XFRY8JPuxsM": "1000000000000000000",
		"3a3ZjpWqX8XsNyW7bRSwPDHhzAY3rEvLeBqwMTSYjTFuWt3e": "12345543210000000000000",
		"3k5j8hTZF5w9fdp2RwKLEqiSXHXLcz1FKbxhYekPfK6Jep73": "731000000000000000000",
		"4FzQKVsh4kucKeJUak4eDwGRdD1biYHGhbVLK2YFH1mDMa56": "1000000000000000000",
		"5kpcAHFCUdDhtUZkg3LeNrRSrd4qDvjewmiqKcWYMfokryTH": "1847000000000000000000",
		"5nFDBSyaHXNGun5pbDSNqxi7F3YpXWK4xrzpXm7ozeSBXEJL": "2000000000000000000",
		"5sbxYdYMEiD1rMpxDYVpdccJ8WSar6qJieBoFwi4FbeG5VJ6": "3277490944134000000000",
		"79SEtu9JcBSTgNoFRY5LDaTS34SE18sdVhTyh3NQmVkUF2mu": "669133425468142987019504",
		"87FXCi33z4PsrmiDDcSDnYPs55Yws5qZxmpsvqRjQP92oK9u": "1000000000000000000",
		"8wdRsUoRGfbe7QbT7pXCFUVzJ3h8DHeT3jRqbGcBa3Pw369p": "649000000000000000000",
		"9K4R2VA2i7Bbc4GefBEG33mtQUNdSELDH48h7gTrHG5jvhsb": "21000000000000000000",
		"9MucsN5Xtrd825LRFhhSAhnzyucnzDx77E7DbWoygnLLDRqx": "2000000000000000000",
		"9W3eVBRzjHyhbszCZYnbapCdvVL9tfQKBr9w9RzvLaNx3vYg": "17779873888540000000000",
		"9Z6egA5rNfdGLePHuKb5v1skG2LQeubhWjWsQkw6vYD8yFRn": "232028000000000000000000",
		"9sS1cdn8GPLG6NfD73FwYTUH6pK7tvaWMwmd7Xd2WA5nsC27": "185472575497271319402752",
		"BAPrS65YAbgK6Sb6vm3BZytramoZSnTkVQeh7GZVkKpG3RMR": "2083814573947194389443",
		"BeaDaq2xVa697RGBu5hZ48EYMdgbNVd1kr9DgZoTFsymY4Y":  "6335115345155000000000",
		"CFeWRU6AJKRKe2GXsP3rKMkypqjvPspEFrZHFDAy5i8bmKo7": "3213100329527000000000",
		"CQiHNh5ip3K8YJpS84PeUFkrwTgkiePsExN4M35Pmu5nj3mG": "19404358913906000000000",
		"D8zB2JNHWiRBaeALgT8mdUBdANAVHBVsLTAo9t21zjSfuCbD": "1000000000000000000",
		"DVY2XUphcqUWpnTABcRynJFqtuXEnMS4TT7b7rtLHGrkXRuT": "3000000000000000000",
		"DfGf8ZtxYuaNuyK4M4UXzZzWMDg34Ze6JMWPcqNAvqDST2N":  "362433999460601809710888",
		"E2YxDQ9AiaueAQzJNZrJuMh7FoeVY6YqyAiJGqgg7q7GQv7Q": "7000000000000000000",
		"EYmzQYAN6veU2jGQFU6oWQBCkS5KaC3ht3eDhGQjUEvW2ndB": "179053653521844647584660",
		"Et73BZBgFWUZiFrHsgeMjRoWYGzahQYAgAy4t5xztLwUmfvA": "37800000000000000",
		"FCM9yMw8VibFUXLKPQGJCmLpHQbZ6tqTiyeaHsWKAdoWuzFU": "9702218059836000000000",
		"FFCMvZybFM5hybvasq8iAXARr9Kjev1X2f2fA5NeZXYHjfsq": "9000000000000000000",
		"Fo7jKqta38RWFDXBQ7Ty4wALq6WdvvCGVpLMVJJECLCsjbpX": "2000000000000000000",
		"GLmNguG25JtqjWEhYwJaTWxtByJDkmYnAEGgSzZTc7FtGKpe": "55483269452432000000000",
		"GQ3VLqbkyivMH1HJ1btxDU41EH8cKjSsjpa4NEuznXTPCcfV": "160407000000000000000000",
		"Gz8jWRZCebFpE2GfQMjbWQ8L5nxGqDom2snia9f5KzPD4vBF": "358642632874769478059978",
		"H4p9rDMjKZtDkiedbG3EcdReExN1Fs5RSSuwbXmAY24d8D5W": "365915661211488927357164",
		"H7xQF64AJ5hSGcYWpEz57dEWb1qfNkVkworpLN1AwfL6nWqT": "7048600540936855739981830",
		"HAAti9NDKvZBrSiGH97etV79Lu8YAyKVTKRW9k7x2dWNkp5Q": "91710735383000000000",
		"HCj4HP46ejgeZWza6QqpWgfjUkiVoQhJc3yQcagHR8HfVJYq": "1000000000000000000",
		"JPGXLLFR3eUYgRnctAVwB2m4RQDgQokFLjyA4QUvRQ3UGLiE": "19995800000000000000",
		"K1PnAY4aidPaSpE8tRtWQsr2Lb6aa2AdFTra2QFP4qTE6tre": "26602000000000000000000",
		"K6tQ2wapR7JZjf91ssK5c5DfCXFPY2Wuz7bwMyT3HsaREwf6": "2500000000000000000000",
		"KX9Grs4iVtL4k9mfMxqUny8UMP2dyYbNuw61JYsHrqE9MbCK": "1254446670000000000000",
		"KYkNSkNSdFRwhqUU44dyWR4VCtziY6d3HwsBZ6ffLCV3V2RJ": "2000000000000000000",
		"KZP5ugr3dqdhPMXrYjBtG2JYZ2w8XS5sJsHZnRUXdWP6HEBo": "52830633055838000000000",
		"KdX2gvCAbm369NViFghau7k9jgc83VAJ2d6PJGehwJ6y4psw": "4000000000000000000",
		"LcNcrpsGZ7XxPUNfw4Wj2YPoENtot57Jo2G2zRXFzzKHFPAW": "1102308224808000000000",
		"LdjHjg5u284JN7nmP8npz5tSLc5CwnJw5Gtrtv24b89CpDMG": "1544000000000000000000",
		"Lo4odLdP1MWpmvq7yKwJ5cLEeRqsNf8d2i5QVwWRNUYV92Ua": "167393260344363631007837",
		"MmY7USp6p1G19kDmDY3RmFy9xsSXadhYgqMt6FhjM5FsD4Aq": "16000000000000000000",
		"N4RZBB6JLwJpPbCPtjy9eKqJe7UoFGCUVrz6YGS6XGn51ApT": "470354102952000000000",
		"NRhMxpew2yDbvcPVRgM28KFd37p9r7nNvMBwbzMuWVseh1Gj": "6000000000000000000",
		"NbJcLWuV4k6JKXbh4ry6FtcRqYLvJZxWT9573Dcco5rLKVUL": "1000000000000000000",
		"NoWe6zia8RLevUC8Fp9WsnxqrtumvNxXE5JcrgqBrcgJHiaH": "19000000000000000000",
		"Np5RpD9QUHDXjyv9LunDtw4eyByzZd7i5ERapjdz73QqqFSy": "122077000000000000000000",
		"NyunbzpfjjB9QpuX7ZuZYvwKEQKEX7fu8EVLyREwq2AS7WT1": "219721260000000000000",
		"P2AtzvtQTimGKH4BeTHhZmVxbUPeh8ctoquTVaocWo8oSBbw": "5591000000000000000000",
		"PS2BkqouhzXG7iUaoDi3TsLi6z18yLHeUirtxm8mdqCeAg9Q": "20406488335845485912235",
		"PVtPy2uWKgD3b2J9wzwMtbeBTxqFQsu1cdQTDYQGnA6uKqo6": "141938196162509269062710",
		"QUHmWrtHqVzzRwHoxeWM6C8ah2m9XPznUkJxLbCuLaYjZdAX": "2497000000000000000000",
		"QVSdpMLFwUtECb3SxgLt8YeQwkHGmzh5ZexjGCUB2E5koFhJ": "1000000000000000000",
		"QfUnsE4CNgnpVS4oC4WEYH8u7WWAs8AwMrFBknWWqGSYwBXU": "12000000000000000000",
		"Qwthbiv6pgLNqkYMbU7cyJiDq4g7vE6VFv2oJ1uaokihXRPA": "10362363463272000000000",
		"RCdbJwCynneCxmQdkh9QY5brqxcZp89oyj9G8JeqASQ4B9Te": "9000000000000000000",
		"RDnTLnnRjj3PucfBzXq8HSoNrC4v7ibMkhv2yXcpR38LVrUh": "16937085075848000000000",
		"RFs611Cswj7Hi3FhYJDFFQWgqtAYwJyhppM23Ck6hgidukoC": "3255000000000000000000",
		"RwMhTnFNj84sSTkiuW9pjD2zoBLGG5RgydcvoXf196MuTVrX": "1997397999999999970",
		"coutNLcFDZ283CxwtQHkFtF5ngAptjEkUSkVVaS6kFT9QCj":  "558281168483068014434913",
	}

	genesisBalances := make([]*Balance, 0, len(strMap))
	for strAddr, strBal := range strMap {
		bal, ok := new(big.Int).SetString(strBal, 10)
		if !ok {
			panic(fmt.Errorf("failed to load balance: %s", strBal))
		}

		addr, err := base58.FastBase58Decoding(strAddr)
		if err != nil {
			panic(err)
		}

		genesisBalances = append(genesisBalances, &Balance{
			Address: addr,
			Amount:  bal,
		})
	}
}

// GetGenesisSheet returns a genesis sheet
func GetGenesisSheet(network Network) *Sheet {
	if genesisSheet == nil {
		accounts := make([]*Account, 0, 100)

		for i := uint64(0); i <= 100; i++ {
			accounts = append(accounts, GetGenesisStyleAccount(AccountNum(i)))
		}

		genesisSheet = NewSheet(network, 0, GetGenesisBlockHash(network), genesisBalances, accounts)
	}

	return genesisSheet
}
