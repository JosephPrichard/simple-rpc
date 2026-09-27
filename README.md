# Power RPC

An RPC protocol over TCP with support for bit-level control.

## Installation

`go get github.com/josephprichard/brpc`

## Usage

Define a data spec using the 'message' structure.

```
message Board struct {
    required turn @1 bint; // a 1 bit field, it can either be white or black's turn
    required color @2 [64]int2; // a 128 bit field that will be deserialized as a 64 length array
}

message Move struct {
    required row @1 int3; // a 3 bit field, since this can only contain values 0-7
    required col @2 int3;
    required disc @3 int1; // a 1 bit field, this can either be white or black
}

message GameResult struct {
    required winnerId @1 int128;
    required loserId @2 int128;
    required winerElo @3 float32; // a float is a 32 or 64 bit float
    required loserElo @4 float32;
    required isDraw @5 int1;
}

message Game struct {
    required id @4 int128; // we can add fields in any order in the spec, the assigned 'tag' value uniquely identifies the field in serialized buffer
    required whiteId @1 int128; // storing a uuid in a 128 bit field
    required blackId @2 int128;
    required board @3 OthelloBoard; // compose a larger message from smaller messages, this is stored like we just copied the fields in here
    required moves @5 []Move; // a variable length array, the size will be packed into 4 bytes at the start of the array
    optional result @6 GameResult; // a field can be optional, in which it may or may not exist in the message
}
```

Define an RPC service using the 'service' structure.
```
// a serivice that performs operations for othello games over the wire
// notice that all arguments and return values are performed through named tuples, this can be thought of as an anonymous message defined for each operation
service OthelloService {
    MakeMove (move Move) -> b8;
    GetGame (id b128) -> (b8, Game);
}
```