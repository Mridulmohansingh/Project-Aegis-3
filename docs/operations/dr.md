# Disaster Recovery & Operations Guide

This guide covers operational configurations for high availability, backups, and disaster recovery.

## Database HA & Replication

AEGIS uses a PostgreSQL cluster deployed across multiple Availability Zones (Multi-AZ):

* **Primary Instance**: Receives all write traffic. Deployed in private subnets.
* **Standby Instance**: Synchronous replication ensures zero data loss (RPO = 0) on failover.
* **Read Replicas**: Distributes query workloads (e.g. administrative dashboard stats).

---

## Partitioning Strategy

To handle millions of concurrent candidate answer submissions on exam day:
* The `candidate_responses` table is hash-partitioned into 64 partitions using the `exam_id` column.
* Indexes are local to partitions, keeping B-Tree depths shallow and insert times bounded.
* Older partition sets can be detached and archived to cold storage post-recalibration.

---

## Backup & Recovery Procedures

* **Automated Snapshots**: Daily EBS snapshots are taken and retained for 35 days (enforced by AWS backup plan or Terraform module).
* **Point-in-Time Recovery (PITR)**: Write-Ahead Logs (WAL) are streamed to secure S3 buckets continuously, allowing recovery to any millisecond within the retention window.
* **Testing Recovery**: Perform dry-run restorations monthly using the automated pipeline.
