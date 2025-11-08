# 🚀 LMS Server - Production Deployment Checklist

## ✅ Pre-Deployment Checklist

### Code & Build

- [x] All code compiles without errors (`go build ./...`)
- [x] All tests pass (`go test ./...`)
- [ ] Code reviewed and approved
- [ ] Version tagged in git (e.g., `v1.0.0`)
- [ ] Build script tested (`./build.sh 1.0.0 docker`)

### Configuration

- [ ] Environment variables set in production `.env`
- [ ] Database credentials secured
- [ ] JWT secrets generated (use secure random strings)
- [ ] Bunny CDN API keys configured
- [ ] Email SMTP settings verified
- [ ] CORS allowed origins set correctly
- [ ] Log level set to `info` or `warn`

### Database

- [ ] Production database created
- [ ] Database migrations run successfully
- [ ] Database indexes created (`scripts/init-db.sql`)
- [ ] Database backups configured
- [ ] Connection pool settings tuned
- [ ] SSL/TLS enabled for database connections

### Security

- [x] Security headers middleware enabled
- [x] Rate limiting configured (100 req/min default)
- [x] Request size limits set (10MB)
- [x] CORS properly configured
- [ ] HTTPS/TLS certificates installed
- [ ] Secrets stored in secure vault (not in code)
- [ ] Database user has minimum required permissions
- [ ] API keys rotated from development values

### Performance

- [x] Response compression enabled (gzip)
- [x] Database query logging configured
- [x] Prepared statements enabled
- [x] Connection pooling optimized
- [x] Cache headers configured
- [ ] CDN configured for static assets
- [ ] Load testing completed

### Monitoring & Observability

- [x] Health check endpoints working (`/health`, `/ready`)
- [x] Prometheus metrics endpoint exposed (`/metrics`)
- [ ] Grafana dashboards imported
- [ ] Log aggregation configured (ELK, CloudWatch, etc.)
- [ ] Error tracking configured (Sentry, etc.)
- [ ] Uptime monitoring configured (Pingdom, UptimeRobot, etc.)
- [ ] Alerts configured for critical metrics

### Infrastructure

- [ ] Docker image built and tagged
- [ ] Docker image pushed to registry
- [ ] Kubernetes manifests configured (if using K8s)
- [ ] Resource limits set (CPU, memory)
- [ ] Horizontal pod autoscaling configured (if K8s)
- [ ] Load balancer configured
- [ ] DNS records configured
- [ ] SSL/TLS termination at load balancer

### Disaster Recovery

- [ ] Database backup strategy defined
- [ ] Recovery time objective (RTO) documented
- [ ] Recovery point objective (RPO) documented
- [ ] Backup restoration tested
- [ ] Rollback plan documented

---

## 🎯 Performance Targets

### Response Times (95th percentile)

- [ ] Health check: < 10ms
- [ ] Database queries: < 100ms
- [ ] API endpoints: < 200ms
- [ ] File uploads: Based on size + network

### Throughput

- [ ] Minimum: 500 requests/second
- [ ] Target: 1000-2000 requests/second
- [ ] Load tested up to 2x expected peak traffic

### Resource Usage

- [ ] Memory: < 512MB per instance
- [ ] CPU: < 50% at average load
- [ ] Database connections: < 80% of pool

### Availability

- [ ] Uptime target: 99.9% (43 minutes downtime/month)
- [ ] Maximum downtime per incident: < 5 minutes
- [ ] Response time SLA: 95% under 200ms

---

## 🐳 Docker Deployment

### Build Image

```bash
# Set version
export VERSION=1.0.0
export GIT_COMMIT=$(git rev-parse --short HEAD)
export BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build
docker build \
  --build-arg VERSION=$VERSION \
  --build-arg GIT_COMMIT=$GIT_COMMIT \
  --build-arg BUILD_TIME=$BUILD_TIME \
  -t your-registry/lms-server:$VERSION \
  -t your-registry/lms-server:latest \
  .

# Push
docker push your-registry/lms-server:$VERSION
docker push your-registry/lms-server:latest
```

### Run Container

```bash
docker run -d \
  --name lms-server \
  -p 8080:8080 \
  --env-file .env.production \
  --restart unless-stopped \
  your-registry/lms-server:latest
```

### Docker Compose

```bash
# Development
docker-compose up lms-server-dev

# Production
docker-compose --profile production up lms-server

# With monitoring
docker-compose --profile monitoring up
```

---

## ☸️ Kubernetes Deployment

### Apply Manifests

```bash
# Create namespace
kubectl create namespace lms

# Apply secrets (create from .env.production)
kubectl create secret generic lms-server-secrets \
  --from-env-file=.env.production \
  -n lms

# Apply all manifests
kubectl apply -f deployments/kubernetes.yaml

# Verify deployment
kubectl get pods -n lms
kubectl get services -n lms
kubectl get ingress -n lms
```

### Scale Deployment

```bash
# Manual scaling
kubectl scale deployment lms-server --replicas=5 -n lms

# Auto-scaling (HPA already configured)
kubectl get hpa -n lms
```

### Monitor Deployment

```bash
# Watch pods
kubectl get pods -n lms -w

# Check logs
kubectl logs -f deployment/lms-server -n lms

# Port forward for local testing
kubectl port-forward svc/lms-server 8080:8080 -n lms
```

---

## 🔍 Post-Deployment Verification

### Health Checks

```bash
# Liveness
curl http://your-domain.com/health
# Expected: {"status":"ok","timestamp":"...","version":"1.0.0"}

# Readiness
curl http://your-domain.com/ready
# Expected: {"status":"ready","timestamp":"...","checks":{"database":"ok"}}

# Version info
curl http://your-domain.com/version
# Expected: {"version":"1.0.0","git_commit":"...","build_time":"..."}
```

### API Endpoints

```bash
# Test public endpoint
curl http://your-domain.com/api/health

# Test authenticated endpoint (with token)
curl -H "Authorization: Bearer YOUR_TOKEN" \
  http://your-domain.com/api/courses
```

### Performance Testing

```bash
# Apache Bench - quick test
ab -n 1000 -c 10 http://your-domain.com/health

# k6 - comprehensive load test
k6 run load-test.js

# wrk - sustained load test
wrk -t12 -c100 -d30s http://your-domain.com/api/courses
```

### Monitoring

```bash
# Prometheus metrics
curl http://your-domain.com/metrics

# Grafana dashboards
open http://grafana.your-domain.com

# Check logs
kubectl logs -f deployment/lms-server -n lms | grep ERROR
```

---

## 📊 Monitoring Dashboards

### Key Metrics to Monitor

#### Application Metrics

- Request rate (requests/second)
- Response time (p50, p95, p99)
- Error rate (4xx, 5xx)
- Active connections
- Memory usage
- CPU usage

#### Database Metrics

- Query duration
- Connection pool usage
- Slow queries (>200ms)
- Deadlocks
- Active transactions

#### Business Metrics

- User registrations
- Course enrollments
- Payment transactions
- Video uploads
- API usage by endpoint

### Alert Thresholds

```yaml
# Prometheus alert rules
alerts:
  - name: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
    severity: critical

  - name: HighResponseTime
    expr: histogram_quantile(0.95, http_request_duration_seconds_bucket) > 1
    severity: warning

  - name: DatabaseConnectionPoolExhaustion
    expr: db_connections_in_use / db_connections_max > 0.9
    severity: warning

  - name: HighMemoryUsage
    expr: process_resident_memory_bytes > 500000000 # 500MB
    severity: warning
```

---

## 🔧 Troubleshooting

### High Memory Usage

```bash
# Check memory stats
curl http://localhost:8080/debug/db-stats

# Get pprof memory profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

### Slow Queries

```bash
# Check slow query logs
kubectl logs deployment/lms-server -n lms | grep "slow query detected"

# View query metrics
curl http://localhost:9090/api/v1/query?query=db_query_duration_seconds_bucket
```

### High Error Rate

```bash
# Check error logs
kubectl logs deployment/lms-server -n lms | grep "level=error"

# Get error metrics
curl http://localhost:9090/api/v1/query?query=rate(http_requests_total{status=~"5.."}[5m])
```

### Database Connection Issues

```bash
# Check connection pool
curl http://localhost:8080/debug/db-stats

# Verify database connectivity
kubectl exec -it deployment/lms-server -n lms -- sh
$ psql -h $LMS_DB_HOST -U $LMS_DB_USER -d $LMS_DB_NAME
```

---

## 🔄 Rollback Procedure

### Kubernetes

```bash
# View rollout history
kubectl rollout history deployment/lms-server -n lms

# Rollback to previous version
kubectl rollout undo deployment/lms-server -n lms

# Rollback to specific revision
kubectl rollout undo deployment/lms-server --to-revision=2 -n lms

# Check rollback status
kubectl rollout status deployment/lms-server -n lms
```

### Docker

```bash
# Stop current container
docker stop lms-server

# Start previous version
docker run -d \
  --name lms-server \
  -p 8080:8080 \
  --env-file .env.production \
  your-registry/lms-server:1.0.0  # Previous version
```

---

## 📞 Support Contacts

### On-Call Rotation

- Primary: [Name] - [Email] - [Phone]
- Secondary: [Name] - [Email] - [Phone]
- Manager: [Name] - [Email] - [Phone]

### Escalation Path

1. Check monitoring dashboards
2. Review recent deployments
3. Check error logs with request IDs
4. Contact on-call engineer
5. Escalate to manager if critical

---

## 📚 Documentation Links

- [Production Optimization Guide](./PRODUCTION_OPTIMIZATION_GUIDE.md)
- [Frontend Migration Guide](./FRONTEND_PRODUCTION_MIGRATION.md)
- [Direct Upload Guide](./FRONTEND_DIRECT_UPLOAD_MIGRATION.md)
- [API Documentation](./swagger-auto-output.json)
- [Type Optimizations](./TYPE_OPTIMIZATIONS_SUMMARY.md)

---

## ✅ Sign-Off

- [ ] Development Lead: ******\_****** Date: **\_\_\_**
- [ ] DevOps Lead: ******\_****** Date: **\_\_\_**
- [ ] QA Lead: ******\_****** Date: **\_\_\_**
- [ ] Product Manager: ******\_****** Date: **\_\_\_**

**Deployment Date**: ******\_\_\_******  
**Deployment By**: ******\_\_\_******  
**Version**: ******\_\_\_******  
**Git Commit**: ******\_\_\_******

---

**Last Updated**: October 31, 2025  
**Version**: 1.0.0
