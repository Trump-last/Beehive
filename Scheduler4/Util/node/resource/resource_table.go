package resource

import (
	"fmt"
	"strconv"

	"github.com/shopspring/decimal"
)

type ResourceType int

const (
	CPU ResourceType = 0
	RAM ResourceType = 1
)

const ResourceNum = 2

type certainResourceTable map[TaskID]decimal.Decimal

type resourceState struct {

	// NodeCapacity : all the resource this node can provide ,it is depends on the physical information of this machine
	NodeCapacity [ResourceNum]decimal.Decimal

	// The information about the allocation of resources
	NodeResourceTable [ResourceNum]certainResourceTable
}

func floats2decimals(fList [ResourceNum]float32) (dList [ResourceNum]decimal.Decimal) {
	for i, f := range fList {
		dList[i] = decimal.NewFromFloat32(f)
	}
	return
}

func decimals2floats(dList [ResourceNum]decimal.Decimal) (fList [ResourceNum]float32) {
	for i, d := range dList {
		temp, err := strconv.ParseFloat(d.String(), 32)
		if err != nil {
			panic(err)
		}
		fList[i] = float32(temp)
	}
	return
}

func newResourceState(localResource [ResourceNum]float32) *resourceState {
	var resource resourceState
	resource.NodeCapacity = floats2decimals(localResource)
	for i := 0; i < ResourceNum; i++ {
		resource.NodeResourceTable[i] = make(certainResourceTable)
	}
	return &resource
}

func (resource *resourceState) getAvailResource() [ResourceNum]decimal.Decimal {
	var result [ResourceNum]decimal.Decimal
	used := resource.getUsedResource()
	for i := 0; i < ResourceNum; i++ {
		result[i] = resource.NodeCapacity[i].Sub(used[i])
	}
	return result
}

func (resource *resourceState) getUsedResource() [ResourceNum]decimal.Decimal {
	var result [ResourceNum]decimal.Decimal
	for i := 0; i < ResourceNum; i++ {
		certainTable := resource.NodeResourceTable[i]
		var OneResourceUsed decimal.Decimal = decimal.NewFromFloat32(0.0)
		for _, v := range certainTable {
			OneResourceUsed = OneResourceUsed.Add(v)
		}
		result[i] = OneResourceUsed
	}
	return result
}

func (resource *resourceState) allocTaskResource(id TaskID, request [ResourceNum]float32) bool {
	available := resourceVec(resource.getAvailResource())
	requestD := floats2decimals(request)
	if available.canCover(requestD) {
		for i := 0; i < ResourceNum; i++ {
			resource.NodeResourceTable[i][id] = requestD[i]
		}
		return true
	} else {
		return false
	}
}

func (resource *resourceState) removeTaskResource(id TaskID) {
	for i := 0; i < ResourceNum; i++ {
		certainTable := resource.NodeResourceTable[i]
		delete(certainTable, id)
	}
}

func (resource *resourceState) String() string {
	var res string
	res += fmt.Sprintln("capacity:", resource.NodeCapacity)
	res += fmt.Sprintln("used:", resource.getUsedResource())
	res += fmt.Sprintln("avail:", resource.getAvailResource())
	for i := 0; i < ResourceNum; i++ {
		res += fmt.Sprintln(resource.NodeResourceTable[i])
	}
	return res
}

type resourceVec [ResourceNum]decimal.Decimal

func (a resourceVec) canCover(b resourceVec) bool {
	for i := 0; i < ResourceNum; i++ {
		if a[i].Cmp(b[i]) < 0 {
			return false
		}
	}
	return true
}

func (a resourceVec) add(b resourceVec) (res resourceVec) {
	for i := 0; i < ResourceNum; i++ {
		res[i] = a[i].Add(b[i])
	}
	return res
}

func (a resourceVec) minus(b resourceVec) (res resourceVec) {
	for i := 0; i < ResourceNum; i++ {
		res[i] = a[i].Sub(b[i])
	}
	return res
}
