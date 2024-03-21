package streams

// MultiClient represents a connected multiClient to the Windows Agent.
// It abstracts away the multiple streams into a single object.
// It only provides communication primitives, it does not handle the logic of the messages themselves.
type MultiClient = multiClient
