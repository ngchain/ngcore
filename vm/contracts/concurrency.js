"use strict";

var owner; // initialized by vm

// Contract is an example that works as a currency
function Contract() {
    this.name = "myCoin"; // const
    this.state = JSON.parse(owner.State);

    // called when init
    this.onInit = function () {
        // assign coins
        this.state.Balances = {};
        this.state.Balances[owner.Num] = 100;
        this.state.Balances[owner.Num + 1] = 100;
        this.state.Balances[owner.Num - 1] = 100;

        return JSON.stringify(this.state);
    }

    // called by tx
    // extra = {[receiver1, amount1], [receiver2, amount2], ...}
    this.onTx = function () {
        var recvPairs = JSON.parse(tx.extra);

        var total;
        for (var pair in recvPairs) {
            total += pair[1]
        }

        if (state.Balances[convener] < total) {
            return JSON.stringify(this.state) // do no change
        }

        this.state.Balances[convener] -= total
        for (var pair in recvPairs) {
            this.state.Balances[pair[0]] += pair[1]
        }

        return JSON.stringify(this.state)
    }

    this.main = function () {
        // owner.State
        return this.state.name // test
    };
}
