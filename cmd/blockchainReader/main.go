package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"go.etcd.io/bbolt"
	"log"
)

var file = flag.String("f", "ngcore.db", "path of ngcore.db file")
var key = flag.String("k", "", "key in hex")
var value = flag.String("v", "", "value in hex")

func main() {
	flag.Parse()
	db, err := bbolt.Open(*file, 0666, &bbolt.Options{ReadOnly: true})
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()
	db.View(func(tx *bbolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("block"))
		if *key != "" {
			k, _ := hex.DecodeString(*key)
			v := b.Get(k)
			fmt.Printf("key=%x, value=%x\n", k, v)
			return nil
		}

		if *value != "" {
			c := b.Cursor()

			raw, err := hex.DecodeString(*value)
			if err != nil {
				log.Panic(err)
			}
			for k, v := c.First(); k != nil; k, v = c.Next() {
				if bytes.Compare(v, raw) == 0 {
					fmt.Printf("key=%x, value=%x\n", k, v)
				}
			}

			return nil
		}

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%x, value=%x\n", k, v)
		}

		return nil
	})
}
