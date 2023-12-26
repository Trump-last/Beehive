/*
 * @Autor: XLF
 * @Date: 2023-03-09 11:24:01
 * @LastEditors: XLF
 * @LastEditTime: 2023-09-05 11:33:23
 * @Description:
 */
package slicemap

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"

	gcsProto "gitee.com/linfeng-xu/protofile/gcsGrpc"
)

const (
	_BASE_NO_SHRINK_SIZE = 10000
)

type SliceMap struct {
	objSlice []*gcsProto.NbhInfo
	idMap    map[string]int
	maxIdx   int
	mu       sync.RWMutex
}

func NewSliceMap() *SliceMap {
	return &SliceMap{
		objSlice: make([]*gcsProto.NbhInfo, 0),
		idMap:    make(map[string]int),
		maxIdx:   0,
		mu:       sync.RWMutex{},
	}
}

func (s *SliceMap) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxIdx
}

func (s *SliceMap) Get(id string) *gcsProto.NbhInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if idx, ok := s.idMap[id]; ok {
		return s.objSlice[idx]
	}
	return nil
}

func (s *SliceMap) GetByidx(idx int) *gcsProto.NbhInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.objSlice[idx%s.maxIdx]
}

func (s *SliceMap) QueryIdx(id string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if idx, ok := s.idMap[id]; ok {
		return idx
	}
	return -1
}

func (s *SliceMap) Del(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.Split(id, "/")[2]
	if idx, ok := s.idMap[id]; ok {
		delete(s.idMap, id)
		if idx == s.maxIdx-1 {
			s.objSlice[idx] = nil
			s.maxIdx--
		} else {
			s.maxIdx--
			obj := s.objSlice[s.maxIdx]
			s.objSlice[idx] = obj
			s.idMap[obj.Addr] = idx
			s.objSlice[s.maxIdx] = nil
		}
	} else {
		return
	}
	fvalue := len(s.objSlice) - s.maxIdx
	if s.maxIdx > _BASE_NO_SHRINK_SIZE && fvalue > 0 &&
		(float32(fvalue)/float32(s.maxIdx) > 0.1) {
		s.shrink()
	}
}

func (s *SliceMap) Add(obj *gcsProto.NbhInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx, ok := s.idMap[obj.Addr]; ok {
		s.objSlice[idx] = obj
		return
	}
	if s.maxIdx == len(s.objSlice) {
		s.objSlice = append(s.objSlice, obj)
	} else {
		s.objSlice[s.maxIdx] = obj
	}
	s.idMap[obj.Addr] = s.maxIdx
	s.maxIdx++
}

func (s *SliceMap) GetSlice(addr string, num int) []*gcsProto.NbhInfo {
	s.mu.RLock()
	defer func() {
		s.mu.RUnlock()
		if err := recover(); err != nil {
			fmt.Println("GetSlice panic", err)
		}
	}()
	var ret []*gcsProto.NbhInfo
	if s.maxIdx <= num {
		return ret
	}
	if idx, ok := s.idMap[addr]; ok {
		left := (idx + s.maxIdx - 1) % s.maxIdx
		right := (idx + 1) % s.maxIdx
		switch num {
		case 0:
			break
		case 1:
			ret = append(ret, s.objSlice[right])
		case 2:
			ret = append(ret, s.objSlice[right])
			ret = append(ret, s.objSlice[left])
		case 3:
			ret = append(ret, s.objSlice[right])
			ret = append(ret, s.objSlice[left])
			index := rand.Intn(s.maxIdx)
			for index == right || index == left || index == idx {
				index = rand.Intn(s.maxIdx)
			}
			ret = append(ret, s.objSlice[index])
		default:
			base := num/2 - 2
			m := make(map[int]struct{})
			m[right] = struct{}{}
			m[left] = struct{}{}
			for i := 0; i < base; i++ {
				if i%2 != 0 {
					left = (left + s.maxIdx - 1) % s.maxIdx
					m[left] = struct{}{}
				} else {
					right = (right + 1) % s.maxIdx
					m[right] = struct{}{}
				}
			}
			k := rand.Intn(s.maxIdx)
			for i := 0; i < num-2-base; i++ {
				for _, ok := m[k]; ok || k == idx; _, ok = m[k] {
					k = rand.Intn(s.maxIdx)
				}
				m[k] = struct{}{}
			}
			for vl := range m {
				ret = append(ret, s.objSlice[vl])
			}
		}
	}
	return ret
}

func (s *SliceMap) GetAll() []*gcsProto.NbhInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.objSlice[:s.maxIdx]
}

func (s *SliceMap) shrink() {
	if s.maxIdx < len(s.objSlice) {
		s.objSlice = s.objSlice[:s.maxIdx]
	}
}
