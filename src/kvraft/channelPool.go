package kvraft

// ChannelPool is a struct that represents a pool of boolean channels
type ChannelPool struct {
	channels []chan int // the slice of channels
	size     int        // the size of the pool
}

// NewChannelPool creates a new ChannelPool with the given size
func NewChannelPool(size int) *ChannelPool {
	// create a slice of channels with the given size
	channels := make([]chan int, size)
	// initialize each channel
	for i := range channels {
		channels[i] = make(chan int, 2)
	}
	// return a pointer to the ChannelPool
	return &ChannelPool{
		channels: channels,
		size:     size,
	}
}

// GetChannel returns a channel from the pool based on a hash of an int32 value
func (cp *ChannelPool) GetChannel(value int32) chan int {
	// create a new hash function
	// h := fnv.New32a()
	// write the value as bytes to the hash function
	// h.Write([]byte(fmt.Sprint(value)))
	// get the hash value as an uint32
	// hash := h.Sum32()
	// get the index of the channel by modding the hash with the size of the pool
	index := int(value) % cp.size
	// return the channel at the index
	return cp.channels[index]
}
