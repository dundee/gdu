package common

// Is64Bit returns true if the system is 64-bit
const Is64Bit = (^uint(0) >> 63) == 1
