// GNU GPL v3 License
// Copyright (c) 2017 github.com:go-trellis

package node

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-trellis/formats"
)

type consistent struct {
	Name   string
	nodes  map[string]*Node
	hashes map[uint32]*Node

	rings formats.Uint32s
	count int64

	sync.RWMutex
}

// NewConsistent get consistent node manager
func NewConsistent(name string) Manager {
	if name == "" {
		return nil
	}
	return &consistent{Name: name}
}

func (p *consistent) IsEmpty() bool {
	return atomic.LoadInt64(&p.count) == 0
}

func (p *consistent) Add(node *Node) {
	if node == nil {
		return
	}
	p.Lock()
	defer p.Unlock()
	p.add(node)
}

func (p *consistent) add(pNode *Node) {
	if p.nodes == nil {
		p.nodes = make(map[string]*Node, 0)
	}
	if p.hashes == nil {
		p.hashes = make(map[uint32]*Node, 0)
	}

	node := p.nodes[pNode.ID]

	if node != nil {
		p.removeByID(pNode.ID)
	}

	p.nodes[pNode.ID] = pNode

	for i := uint32(0); i < pNode.Weight; i++ {
		crc32Hash := p.genKey(pNode.ID, int(i+1))
		println(pNode.ID, i, crc32Hash)
		if p.hashes[crc32Hash] == nil {
			vnode := *pNode
			vnode.number = i + 1
			p.hashes[crc32Hash] = &vnode
			p.count++
		}
	}

	p.updateRings()

	return
}

func (p *consistent) NodeFor(keys ...string) (*Node, bool) {
	if len(keys) == 0 || p.IsEmpty() {
		return nil, false
	}
	p.RLock()
	defer p.RUnlock()

	return p.hashes[p.rings[p.search(crc32.ChecksumIEEE([]byte(strings.Join(keys, "::"))))]], true
}

func (p *consistent) search(key uint32) (i int) {
	f := func(x int) bool {
		return p.rings[x] > key
	}
	i = sort.Search(int(p.count), f)
	if i >= int(p.count) {
		i = 0
	}
	return
}

func (p *consistent) Remove() {
	p.Lock()
	defer p.Unlock()
	p.remove()
}

func (p *consistent) remove() {
	p.hashes = nil
	p.nodes = nil
	p.updateRings()
}

func (p *consistent) RemoveByID(id string) {
	p.Lock()
	defer p.Unlock()
	p.removeByID(id)
}

func (p *consistent) removeByID(id string) {
	if p.nodes == nil {
		return
	} else if p.IsEmpty() {
		p.remove()
		return
	}

	node := p.nodes[id]
	if node == nil {
		return
	}

	for i := uint32(0); i < node.Weight; i++ {
		crc32Hash := p.genKey(id, int(i+1))
		if value := p.hashes[crc32Hash]; value == nil {
			continue
		} else {
			if value.ID != id {
				continue
			}
		}
		delete(p.hashes, crc32Hash)
		p.count--
	}

	delete(p.nodes, id)
	p.updateRings()
}

func (p *consistent) updateRings() {
	p.count = int64(len(p.hashes))
	if p.count == 0 {
		return
	}

	rings := p.rings[:0]
	//reallocate if we're holding on to too much (1/4th)
	if int64(cap(p.rings))/(p.count*4) > p.count {
		rings = nil
	}
	for k := range p.hashes {
		rings = append(rings, k)
	}
	sort.Sort(rings)
	p.rings = rings
}

func (p *consistent) genKey(elt string, idx int) uint32 {
	return crc32.ChecksumIEEE([]byte(p.Name + "::" + elt + "::" + strconv.Itoa(idx)))
}

func (p *consistent) PrintNodes() {
	p.RLock()
	defer p.RUnlock()

	for i, v := range p.nodes {
		fmt.Println("nodes:", i, *v)
	}
	for i, v := range p.hashes {
		fmt.Printf("hashes: %11.d: %v\n", i, *v)
	}
}
