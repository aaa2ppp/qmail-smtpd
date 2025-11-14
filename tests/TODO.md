# Test Automation TODO

## Current Problem
Test results require manual verification - tests run but don't validate output automatically.

## Priority Tasks

### 1. SMTP Protocol Tests (`tests/test1/`)
- [ ] Add expected response validation to `test.sh`
- [ ] Compare actual responses against expected patterns
- [ ] Validate queue output files (`qq.out0`, `qq.out1`)
- [ ] Check for specific error codes and success responses

### 2. TLS Tests (`tests/tls/`)
- [ ] Automate TLS handshake verification
- [ ] Validate certificate handling
- [ ] Test STARTTLS command flow

### 3. TCP Rules Tests (`tests/tcprules/`)  
- [ ] Validate CDB generation from rules
- [ ] Test IP address matching logic
- [ ] Verify environment variable expansion
- [ ] Check deny/allow rules execution

### 4. Test Infrastructure
- [ ] Create test runner with exit codes
- [ ] Add diff-based validation
- [ ] Set up CI-friendly test reporting
