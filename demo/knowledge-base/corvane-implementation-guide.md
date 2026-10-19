# Corvane Implementation Guide

A typical Enterprise implementation takes 6 to 8 weeks from contract signature to
go-live, delivered by a Corvane implementation lead and a solutions engineer.

## Phases

1. **Discovery (weeks 1-2).** Workshops to confirm data sources, reporting
   requirements, user roles and success criteria. Output: an approved
   implementation plan.
2. **Configuration and integration (weeks 3-5).** Connect data sources, configure
   SSO and SCIM, set up roles, and migrate existing reports.
3. **Validation (week 6).** The customer's team validates data accuracy and user
   acceptance testing is completed.
4. **Training and go-live (weeks 7-8).** Role-based training, cutover, and two
   weeks of hypercare support after go-live.

## Integrations

Corvane includes more than 150 prebuilt connectors, including Snowflake,
PostgreSQL, Microsoft SQL Server, Salesforce, Workday, ServiceNow and Amazon S3.
Custom sources can be connected through the REST API or by dropping CSV and
Parquet files into a managed SFTP location.

The REST API is documented with OpenAPI and allows 1,000 requests per minute per
organization by default.

## Data migration

Historical data is loaded through the same connectors used for ongoing
synchronization. Corvane's implementation team migrates up to 50 existing
dashboards as part of the standard engagement; additional dashboards are scoped
separately.

## Training

Training is delivered live online in role-based sessions: administrators
(one 3-hour session), report builders (two 2-hour sessions) and business users
(one 90-minute session). Sessions are recorded, and the self-paced Corvane
Academy is included for all users at no additional cost.
