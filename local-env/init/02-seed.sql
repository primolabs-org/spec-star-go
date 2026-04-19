-- Deterministic seed data for local development.
-- Safe to re-apply: all INSERTs use ON CONFLICT DO NOTHING.

INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000001', 'CLIENT-001') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000002', 'CLIENT-002') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000003', 'CLIENT-003') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000004', 'CLIENT-004') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000005', 'CLIENT-005') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000006', 'CLIENT-006') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000007', 'CLIENT-007') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000008', 'CLIENT-008') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000009', 'CLIENT-009') ON CONFLICT (client_id) DO NOTHING;
INSERT INTO clients (client_id, external_id) VALUES ('00000000-0000-0000-0000-000000000010', 'CLIENT-010') ON CONFLICT (client_id) DO NOTHING;

INSERT INTO assets (asset_id, instrument_id, product_type, emission_entity_id, asset_name, issuance_date, maturity_date) VALUES ('00000000-0000-0000-0000-000000000101', 'CDB-0001', 'CDB', 'EMISSOR-LOCAL', 'CDB Local Test', '2026-01-01', '2030-01-01') ON CONFLICT (asset_id) DO NOTHING;
INSERT INTO assets (asset_id, instrument_id, product_type, emission_entity_id, asset_name, issuance_date, maturity_date) VALUES ('00000000-0000-0000-0000-000000000102', 'LF-0001', 'LF', 'EMISSOR-LOCAL', 'LF Local Test', '2026-01-01', '2030-01-01') ON CONFLICT (asset_id) DO NOTHING;
INSERT INTO assets (asset_id, instrument_id, product_type, emission_entity_id, asset_name, issuance_date, maturity_date) VALUES ('00000000-0000-0000-0000-000000000103', 'LCI-0001', 'LCI', 'EMISSOR-LOCAL', 'LCI Local Test', '2026-01-01', '2030-01-01') ON CONFLICT (asset_id) DO NOTHING;
INSERT INTO assets (asset_id, instrument_id, product_type, emission_entity_id, asset_name, issuance_date, maturity_date) VALUES ('00000000-0000-0000-0000-000000000104', 'LCA-0001', 'LCA', 'EMISSOR-LOCAL', 'LCA Local Test', '2026-01-01', '2030-01-01') ON CONFLICT (asset_id) DO NOTHING;
INSERT INTO assets (asset_id, instrument_id, product_type, emission_entity_id, asset_name, issuance_date, maturity_date) VALUES ('00000000-0000-0000-0000-000000000105', 'CRI-0001', 'CRI', 'EMISSOR-LOCAL', 'CRI Local Test', '2026-01-01', '2030-01-01') ON CONFLICT (asset_id) DO NOTHING;
INSERT INTO assets (asset_id, instrument_id, product_type, emission_entity_id, asset_name, issuance_date, maturity_date) VALUES ('00000000-0000-0000-0000-000000000106', 'CRA-0001', 'CRA', 'EMISSOR-LOCAL', 'CRA Local Test', '2026-01-01', '2030-01-01') ON CONFLICT (asset_id) DO NOTHING;
INSERT INTO assets (asset_id, instrument_id, product_type, emission_entity_id, asset_name, issuance_date, maturity_date) VALUES ('00000000-0000-0000-0000-000000000107', 'LFT-0001', 'LFT', 'EMISSOR-LOCAL', 'LFT Local Test', '2026-01-01', '2030-01-01') ON CONFLICT (asset_id) DO NOTHING;
