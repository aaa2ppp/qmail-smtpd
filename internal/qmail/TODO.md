# Queue Testing TODO

## Critical Gaps
Currently no unit tests for queue functionality - high risk of regressions.

## Test Coverage Needed

### 1. Queue Creation
- [ ] Test `Begin()` with valid/invalid inputs
- [ ] Verify envelope file creation
- [ ] Test mailFrom and rcptTo persistence

### 2. Message Writing  
- [ ] Test `Write()` with various data sizes
- [ ] Verify message file creation
- [ ] Test error handling (disk full, permissions)

### 3. Queue Commit
- [ ] Test `Commit()` success flow
- [ ] Verify queue directory structure
- [ ] Test rollback on failures

### 4. Edge Cases
- [ ] Empty recipient list
- [ ] Very long email addresses
- [ ] Special characters in envelopes
- [ ] Concurrent queue access

### 5. Integration Tests
- [ ] End-to-end SMTP to queue flow
- [ ] Verify qmail-queue compatibility
- [ ] Test with actual qmail queue dir structure
