Contract = {num = 0}

function Contract.add(a, b) Contract.num = a + b end

function Contract.print(str) print('hello' .. str) end

function Contract.main(self)
    Contract.add(1, 2)
    Contract.print('world')
    Contract.print(Contract.num)
    return Contract.num
end
