'use client';

import * as React from 'react';
import { useParams } from 'next/navigation';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { approvalApi, releaseApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { Badge } from '@/components/ui/badge';
import { Check, X } from 'lucide-react';

export default function ApprovalDetailPage() {
  const params = useParams<{ releaseId: string }>();
  const releaseId = params.releaseId;
  const queryClient = useQueryClient();
  const [comment, setComment] = React.useState('');
  const [userId, setUserId] = React.useState('admin-1');

  const { data: release } = useQuery({
    queryKey: ['release', releaseId],
    queryFn: () => releaseApi.get(releaseId!).then((r) => r.data),
    enabled: !!releaseId,
  });

  const { data: approvals } = useQuery({
    queryKey: ['approvals', releaseId],
    queryFn: () => approvalApi.list(releaseId!).then((r) => r.data),
    enabled: !!releaseId,
  });

  const vote = useMutation({
    mutationFn: (decision: 'approve' | 'reject') =>
      approvalApi.vote(releaseId!, { decision, comment }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['approvals', releaseId] }),
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Release Approval</h1>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Release {releaseId?.slice(0, 8)}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {release ? (
            <>
              <div><span className="text-muted-foreground">Environment:</span> {release.environment}</div>
              <div><span className="text-muted-foreground">Status:</span> <Badge>{release.status}</Badge></div>
            </>
          ) : (
            <div className="text-muted-foreground">Loading release...</div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Existing Approvals ({approvals?.length ?? 0})</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          {approvals?.map((a: { userId: string; decision: string; comment?: string }, i: number) => (
            <div key={i} className="flex items-center justify-between border-b pb-1">
              <span className="font-mono text-xs">{a.userId}</span>
              <Badge variant={a.decision === 'approve' ? 'success' : 'destructive'}>{a.decision}</Badge>
              {a.comment && <span className="text-xs text-muted-foreground">{a.comment}</span>}
            </div>
          ))}
          {(!approvals || approvals.length === 0) && (
            <div className="text-muted-foreground text-center py-2">No votes yet</div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Cast Vote</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <div>
            <label className="text-xs text-muted-foreground">User ID</label>
            <input
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
              className="w-full border rounded-md px-3 py-1.5 text-sm"
            />
          </div>
          <Textarea
            placeholder="Comment (optional)"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            className="h-20"
          />
          <div className="flex gap-2">
            <Button onClick={() => vote.mutate('approve')} disabled={!userId || vote.isPending}>
              <Check className="mr-2 h-4 w-4" />Approve
            </Button>
            <Button variant="destructive" onClick={() => vote.mutate('reject')} disabled={!userId || vote.isPending}>
              <X className="mr-2 h-4 w-4" />Reject
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}