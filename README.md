# raft-go

This is an implementation of raft consensus in Golang according to the official paper - https://raft.github.io/raft.pdf.

The implementation is supposed to be done in 3 parts:
* Leader election
* Log Replication
* Enhancements
  * Cluster membership change
  * Election constraints
  * Log Compaction
