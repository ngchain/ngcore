"use strict";

function Contract() {
    this.num = 0;
    this.add = function (a, b) {
        this.num = a + b;
        return this.num;
    };
    this.main = function () {
        return this.add(1, 2);
    };
}
