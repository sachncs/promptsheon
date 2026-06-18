# Troubleshooting Guide

## Common Issues

### Server Won't Start

**Port already in use:**
```bash
# Find process using port 8080
lsof -i :8080
# Kill the process
kill -9 <PID>
```

**Database locked:**
```bash
# Remove lock file
rm -f data/promptsheon.db-shm data/promptsheon.db-wal
```

### Authentication Errors

**401 Unauthorized:**
- Ensure you're sending the API key in the `Authorization: Bearer <key>` header
- API keys must start with `ps_`
- API key query parameter is disabled by default for security

**Auth disabled (development only):**
```bash
export PROMPTSHEON_AUTH=false
# or
export PROMPTSHEON_AUTH=0
# or
export PROMPTSHEON_AUTH=no
```

### LLM Provider Issues

**Rate limit errors:**
- Configure provider-specific API keys
- Use environment variables: `PROMPTSHEON_OPENAI_API_KEY`, etc.
- Rate limits are configurable in config

**Model not found:**
- Verify the model name is correct for your provider
- Check provider documentation for supported models

### Database Issues

**SQLite database corruption:**
```bash
# Backup and recreate
cp data/promptsheon.db data/promptsheon.db.backup
rm data/promptsheon.db
# Restart server to recreate
```

**Permission denied:**
```bash
chmod 755 data/
chmod 644 data/promptsheon.db
```

### Memory Issues

**High memory usage:**
- Reduce `PROMPTSHEON_SERVER_WRITE_TIMEOUT` and `PROMPTSHEON_SERVER_READ_TIMEOUT`
- Monitor active workflow executions
- Check for memory leaks in long-running processes

### Network Issues

**Connection refused:**
- Verify server is running: `curl http://localhost:8080/health`
- Check firewall rules
- Ensure correct port configuration

**TLS/SSL errors:**
```bash
# Generate self-signed certificate for development
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

## Debug Mode

Enable debug logging:
```bash
export PROMPTSHEON_LOG_LEVEL=debug
export PROMPTSHEON_LOG_FORMAT=json
```

## Health Checks

```bash
# Basic health
curl http://localhost:8080/health

# Readiness (includes DB check)
curl http://localhost:8080/ready
```

## Logs Location

Default log location: `data/promptsheon.log`

Check logs:
```bash
tail -f data/promptsheon.log
```

## Performance Tuning

**Increase file descriptor limit:**
```bash
ulimit -n 65536
```

**Optimize SQLite:**
```bash
export PROMPTSHEON_DB_BUSY_TIMEOUT=5000
export PROMPTSHEON_DB_CACHE_SIZE=-64000
```

## Getting Help

- GitHub Issues: https://github.com/sachn-cs/promptsheon/issues
- Documentation: https://promptsheon.dev/docs
