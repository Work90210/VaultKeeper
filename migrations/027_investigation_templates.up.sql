BEGIN;

CREATE TABLE investigation_templates (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_type       TEXT NOT NULL CHECK (template_type IN (
                            'investigation_plan', 'threat_assessment', 'digital_landscape'
                        )),
    name                TEXT NOT NULL,
    description         TEXT,
    version             INTEGER NOT NULL DEFAULT 1,
    is_default          BOOLEAN NOT NULL DEFAULT false,
    schema_definition   JSONB NOT NULL,
    created_by          UUID,
    is_system_template  BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE investigation_template_instances (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id         UUID NOT NULL REFERENCES investigation_templates(id) ON DELETE RESTRICT,
    case_id             UUID NOT NULL REFERENCES cases(id) ON DELETE RESTRICT,
    content             JSONB NOT NULL,
    status              TEXT NOT NULL DEFAULT 'draft' CHECK (status IN (
                            'draft', 'active', 'completed', 'archived'
                        )),
    prepared_by         UUID NOT NULL,
    approved_by         UUID,
    approved_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_templates_type ON investigation_templates(template_type);
CREATE INDEX idx_template_instances_case ON investigation_template_instances(case_id);
CREATE INDEX idx_template_instances_template ON investigation_template_instances(template_id);

-- Seed default templates
INSERT INTO investigation_templates (template_type, name, description, is_default, is_system_template, schema_definition) VALUES
('investigation_plan', 'Berkeley Protocol Investigation Plan', 'Annex 1: Standard investigation plan template aligned with the Berkeley Protocol.', true, true, '{
    "sections": [
        {"key": "objective", "title": "Investigation Objective", "description": "Define the purpose and goals of the investigation.", "required": true},
        {"key": "scope", "title": "Scope", "description": "Define boundaries: geographic, temporal, subject matter.", "required": true},
        {"key": "methodology", "title": "Methodology", "description": "Describe the investigative approach and techniques to be used.", "required": true},
        {"key": "resources", "title": "Resources Required", "description": "Personnel, tools, platforms, budget, and time allocation.", "required": false},
        {"key": "timeline", "title": "Timeline", "description": "Key milestones and deadlines.", "required": false},
        {"key": "ethical_considerations", "title": "Ethical Considerations", "description": "Privacy, do-no-harm, informed consent, and data protection.", "required": true},
        {"key": "risk_assessment", "title": "Risk Assessment", "description": "Risks to investigators, subjects, and the investigation itself.", "required": true},
        {"key": "data_management", "title": "Data Management", "description": "Collection, storage, access controls, and retention policies.", "required": true},
        {"key": "communication", "title": "Communication Protocols", "description": "Secure communication channels and reporting procedures.", "required": false},
        {"key": "review_schedule", "title": "Review Schedule", "description": "Planned review points and criteria for reassessment.", "required": false}
    ]
}'::jsonb),
('threat_assessment', 'Berkeley Protocol Digital Threat Assessment', 'Annex 2: Digital threat and risk assessment template.', true, true, '{
    "sections": [
        {"key": "threat_actors", "title": "Threat Actors", "description": "Identify potential adversaries and their capabilities.", "required": true},
        {"key": "digital_footprint", "title": "Digital Footprint Risks", "description": "Assess risks from investigator digital presence.", "required": true},
        {"key": "device_security", "title": "Device Security", "description": "Evaluate device configurations, encryption, and physical security.", "required": true},
        {"key": "account_security", "title": "Account Security", "description": "Assess account hygiene, 2FA, separation of identities.", "required": true},
        {"key": "network_security", "title": "Network Security", "description": "VPN, Tor, DNS, and ISP exposure risks.", "required": true},
        {"key": "physical_security", "title": "Physical Security", "description": "Physical location risks and countermeasures.", "required": false},
        {"key": "mitigation", "title": "Mitigation Measures", "description": "Specific countermeasures for identified threats.", "required": true},
        {"key": "incident_response", "title": "Incident Response", "description": "Procedures if compromise is detected.", "required": true},
        {"key": "monitoring", "title": "Monitoring Plan", "description": "Ongoing monitoring for indicators of compromise.", "required": false}
    ]
}'::jsonb),
('digital_landscape', 'Berkeley Protocol Digital Landscape Assessment', 'Annex 3: Digital landscape assessment template for online investigations.', true, true, '{
    "sections": [
        {"key": "platforms", "title": "Platforms in Scope", "description": "Social media platforms, websites, forums, and messaging apps relevant to the investigation.", "required": true},
        {"key": "data_types", "title": "Data Types Available", "description": "Types of data accessible on each platform (posts, profiles, metadata, etc.).", "required": true},
        {"key": "access_methods", "title": "Access Methods", "description": "How data will be accessed: public browsing, API, scraping tools, legal requests.", "required": true},
        {"key": "legal_frameworks", "title": "Legal Frameworks", "description": "Applicable laws, terms of service, and jurisdictional considerations.", "required": true},
        {"key": "technical_constraints", "title": "Technical Constraints", "description": "Rate limits, geoblocking, authentication requirements, and platform changes.", "required": true},
        {"key": "preservation", "title": "Preservation Strategies", "description": "Methods for preserving content that may be deleted.", "required": true},
        {"key": "tools", "title": "Tool Requirements", "description": "Software and tools needed for data collection and analysis.", "required": false},
        {"key": "languages", "title": "Language Considerations", "description": "Languages encountered and translation resources needed.", "required": false}
    ]
}'::jsonb);

COMMIT;
