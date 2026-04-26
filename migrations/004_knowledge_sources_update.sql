-- Clear old default sources and replace with Facebook gray-hat knowledge sources
DELETE FROM knowledge_chunks;
DELETE FROM knowledge_sources;

INSERT INTO knowledge_sources (name, url, source_type, crawl_frequency) VALUES
    -- Official Facebook policies (know the rules to understand violations)
    ('Meta Advertising Standards', 'https://transparency.meta.com/policies/ad-standards', 'official', 'weekly'),
    ('Facebook Community Standards', 'https://transparency.meta.com/policies/community-standards', 'official', 'weekly'),
    ('Meta Business Help - Restricted Content', 'https://www.facebook.com/business/help/299329604655789', 'official', 'weekly'),
    ('Meta Business Help - Account Quality', 'https://www.facebook.com/business/help/2141325395872395', 'official', 'weekly'),

    -- Thai Facebook advertising communities & forums
    ('Pantip - Facebook Ads Forum', 'https://pantip.com/tag/FacebookAds', 'community', 'daily'),
    ('Pantip - ขายของออนไลน์', 'https://pantip.com/tag/ขายของออนไลน์', 'community', 'daily'),

    -- Gray-hat knowledge: account issues, bans, recovery
    ('Facebook Ad Account Disabled Guide', 'https://adespresso.com/blog/facebook-ad-account-disabled/', 'practitioner', 'weekly'),
    ('Facebook Business Manager Restricted', 'https://jonloomer.com/facebook-business-manager-restricted/', 'practitioner', 'weekly'),
    ('Facebook Ads Policy Violations', 'https://www.socialmediaexaminer.com/how-to-avoid-facebook-ad-disapprovals/', 'practitioner', 'weekly'),

    -- Reddit communities for real pain points
    ('r/FacebookAds - Gray Area', 'https://reddit.com/r/FacebookAds', 'community', 'daily'),
    ('r/dropshipping', 'https://reddit.com/r/dropshipping', 'community', 'daily'),
    ('r/Affiliatemarketing', 'https://reddit.com/r/Affiliatemarketing', 'community', 'weekly')
;
