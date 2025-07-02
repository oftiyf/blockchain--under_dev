package rpc


//调用MinnerRPC在block_maker中的MinnerRPC方法
import (
	"blockchain/common"
	"blockchain/maker"
	"fmt"
)

//别人传给我一个maker，我去调用，然后广播即可
func MinnerRPC(maker *maker.BlockMaker,minner common.Address) {
	height:=maker.MinnerRPC(minner)
	fmt.Println("minner",minner,"打包成功，高度为",height)
}