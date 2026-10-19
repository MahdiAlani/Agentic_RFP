# Corvane Service Level Agreement and Support

## Availability

Corvane commits to 99.95% monthly uptime for the Enterprise plan and 99.9% for
the Team plan, excluding scheduled maintenance. Scheduled maintenance is
announced at least 7 days in advance and takes place on Sundays between 02:00 and
04:00 in the customer's region.

If monthly uptime falls below the commitment, customers receive service credits:
10% of the monthly fee below the committed level, 25% below 99.0%, and 50% below
95.0%. Credits are applied to the next invoice on request.

Live and historical availability is published at status.corvane.example.

## Backups and disaster recovery

Customer data is backed up every hour. Backups are encrypted, stored in a
separate availability zone within the customer's selected region, and retained
for 30 days.

Corvane's disaster recovery plan targets a recovery point objective (RPO) of
1 hour and a recovery time objective (RTO) of 4 hours. The plan is exercised
twice a year with a full failover test.

## Support

| Plan       | Hours                  | Channels                 |
|------------|------------------------|--------------------------|
| Team       | Business hours, Mon-Fri | Email, in-app chat       |
| Enterprise | 24/7/365               | Email, chat, phone       |

Response time targets depend on severity:

- Priority 1 (production down): 30 minutes on Enterprise, 4 business hours on Team
- Priority 2 (major feature impaired): 2 hours on Enterprise, 1 business day on Team
- Priority 3 (minor issue or question): 1 business day on both plans

Enterprise customers are assigned a named Customer Success Manager and receive
quarterly business reviews.
