# Corvane Security Overview

Corvane Data Cloud is a managed analytics platform. This document describes how
Corvane protects customer data. It is reviewed every quarter by the Corvane
security team and was last updated in August 2026.

## Encryption

All customer data is encrypted at rest with AES-256. Encryption keys are managed
in AWS Key Management Service (KMS) and rotated automatically every 90 days.
Enterprise customers can supply their own key (BYOK) through KMS.

Data in transit is encrypted with TLS 1.2 or higher; TLS 1.3 is preferred and
used by default for all browser and API traffic. Connections using older
protocols are refused.

## Certifications and audits

Corvane holds a SOC 2 Type II report covering the Security, Availability and
Confidentiality trust services criteria. The most recent audit period ended in
February 2026 and was performed by an independent CPA firm. Corvane is also
ISO/IEC 27001:2022 certified.

An independent third party performs a penetration test of the platform at least
once a year, plus after any major architectural change. Executive summaries of
the latest report are available under NDA.

## Identity and access

Customers can sign in with single sign-on over SAML 2.0 or OpenID Connect,
including Okta, Microsoft Entra ID and Google Workspace. User provisioning and
deprovisioning is supported through SCIM 2.0. Multi-factor authentication can be
enforced for every user in an organization.

Access inside the product is controlled by role-based access control (RBAC) with
five built-in roles (Owner, Admin, Editor, Analyst and Viewer) and custom roles
on the Enterprise plan.

Every administrative action and data export is written to an audit log. Audit
logs are retained for one year and can be streamed to the customer's SIEM.

## Data residency

Customers choose the region their data is stored in when the organization is
created. Available regions are United States (us-east-1), European Union
(eu-central-1) and Canada (ca-central-1). Customer data does not leave the
selected region, including backups.

## Incident response

Corvane maintains a documented incident response plan that is tested twice a
year. Customers are notified of a security incident affecting their data within
24 hours of confirmation.

## Personnel security

All employees complete a background check before starting and security
awareness training every year. Production access requires hardware security
keys and is reviewed quarterly.
