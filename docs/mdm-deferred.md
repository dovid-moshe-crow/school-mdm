# Deferred MDM features

Intentionally **not** yet in school-mdm:

- Declarative Device Management (DDM) full PG backend
- NanoCMD workflow engine / worker
- Full nanok `/api/v1` surface clone

## Now available (ported from nanok)

- DEP/ABM via NanoDEP under `/dep/*` (Bearer or Basic admin auth)
- Thin product API: `/api/mdm/abm/*` (account, sync, define/assign profile)
- DeviceLock + bulk device ops under `/api/mdm/devices/*`

School `/api` remains the product API; thin `/api/mdm/*` covers enqueue ops.
