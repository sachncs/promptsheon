import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { CostBudgetRepo } from '../repos/budget.js';
import type { CostForecastService } from '../analysis/forecast.js';
import { parseBody, parseQuery } from './validate.js';
import { NotFoundError } from '@promptsheon/shared';

const CreateBudgetSchema = z.object({
  organizationId: z.string().min(1),
  label: z.string().min(1).max(120),
  period: z.enum(['weekly', 'monthly']).optional().default('monthly'),
  limitMicros: z.number().int().min(1),
  alertThreshold: z.number().min(0.01).max(1).optional().default(0.8),
  enabled: z.boolean().optional().default(true),
});

const UpdateBudgetSchema = z.object({
  label: z.string().min(1).max(120).optional(),
  period: z.enum(['weekly', 'monthly']).optional(),
  limitMicros: z.number().int().min(1).optional(),
  alertThreshold: z.number().min(0.01).max(1).optional(),
  enabled: z.boolean().optional(),
});

const ForecastQuerySchema = z.object({
  organizationId: z.string().min(1),
  windowDays: z.coerce.number().int().min(7).max(180).optional().default(30),
});

const BudgetListQuerySchema = z.object({
  organizationId: z.string().min(1).max(200),
});

export interface BudgetDeps {
  budgetRepo: CostBudgetRepo;
  forecastService: CostForecastService;
}

function activeOrg(request: { orgContext?: { orgId?: string }; agentOrgId?: string }): string | undefined {
  return request.orgContext?.orgId ?? request.agentOrgId;
}

function assertOrgScope(
  request: { orgContext?: { orgId?: string }; agentOrgId?: string },
  requestedOrgId: string,
  reply: { code: (status: number) => { send: (body: unknown) => unknown } },
): boolean {
  const current = activeOrg(request);
  if (current && current !== requestedOrgId) {
    void reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'organization not found' } });
    return false;
  }
  return true;
}

export function registerBudgetRoutes(app: FastifyInstance, deps: BudgetDeps): void {
  /**
   * List the org's budgets.
   */
  app.get('/api/admin/budgets', async (request, reply) => {
    const parsed = parseQuery(reply, BudgetListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { organizationId: orgId } = parsed.data;
    if (!assertOrgScope(request, orgId, reply)) return;
    return reply.send({ items: deps.budgetRepo.listForOrg(orgId) });
  });

  app.post('/api/admin/budgets', async (request, reply) => {
    const parsed = parseBody(reply, CreateBudgetSchema, request.body);
    if (!parsed.ok) return;
    if (!assertOrgScope(request, parsed.data.organizationId, reply)) return;
    try {
      const created = deps.budgetRepo.create(parsed.data);
      return reply.code(201).send(created);
    } catch (err) {
      const msg = (err as Error).message;
      if (msg.includes('UNIQUE')) {
        return reply.code(409).send({
          error: { code: 'BUDGET_DUPLICATE', message: 'a budget with this label already exists for the org' },
        });
      }
      if (msg.includes('FOREIGN KEY')) {
        return reply.code(404).send({
          error: { code: 'ORG_NOT_FOUND', message: 'organizationId does not reference an existing org' },
        });
      }
      throw err;
    }
  });

  app.patch('/api/admin/budgets/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateBudgetSchema, request.body);
    if (!parsed.ok) return;
    const existing = deps.budgetRepo.findById(id);
    if (!existing) throw new NotFoundError('budget', id);
    if (!assertOrgScope(request, existing.organizationId, reply)) return;
    const updated = deps.budgetRepo.update(id, parsed.data);
    if (!updated) {
      throw new NotFoundError('budget', id);
    }
    return reply.send(updated);
  });

  app.delete('/api/admin/budgets/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const existing = deps.budgetRepo.findById(id);
    if (!existing) throw new NotFoundError('budget', id);
    if (!assertOrgScope(request, existing.organizationId, reply)) return;
    if (!deps.budgetRepo.delete(id)) {
      throw new NotFoundError('budget', id);
    }
    return reply.code(204).send();
  });

  /**
   * Cost forecast: regression over the last N days of cost rollups,
   * projected to month-end with a 95% confidence band. Returns
   * the alerts that would fire right now.
   */
  app.get('/api/admin/cost-forecast', async (request, reply) => {
    const parsed = parseQuery(reply, ForecastQuerySchema, request.query);
    if (!parsed.ok) return;
    if (!assertOrgScope(request, parsed.data.organizationId, reply)) return;
    const result = deps.forecastService.compute(parsed.data.organizationId, {
      windowDays: parsed.data.windowDays,
    });
    if (!result) {
      return reply.send({ snapshot: null, alerts: [] });
    }
    return reply.send(result);
  });
}
