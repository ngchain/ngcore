package main

import (
	"fmt"
	"go.etcd.io/bbolt"
	"log"
)

func main() {
	db, err := bbolt.Open("ngcore.db", 0666, &bbolt.Options{ReadOnly: true})
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()
	db.View(func(tx *bbolt.Tx) error {
		// Assume bucket exists and has keys
		b := tx.Bucket([]byte("vault"))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%x, value=%x\n", k, v)
		}

		return nil
	})
}
