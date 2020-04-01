package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
)

//编码生成公钥字节数组，参数是椭圆曲线对象、x坐标、y坐标等[]byte参数
func ECDSAPublicKey2Bytes(pk ecdsa.PublicKey) []byte {
	return elliptic.Marshal(elliptic.P256(), pk.X, pk.Y)
}

// []byte -> 公钥
func Bytes2ECDSAPublicKey(data []byte) ecdsa.PublicKey {
	x, y := elliptic.Unmarshal(elliptic.P256(), data)
	return ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}
}
