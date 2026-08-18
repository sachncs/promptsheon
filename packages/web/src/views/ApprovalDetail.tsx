import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { approvalApi } from '@/lib/api';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { useState } from 'react';

export function ApprovalDetail() {
  const { releaseId } = useParams<{ releaseId: string }>();
  const queryClient = useQueryClient();
  const [comment, setComment] = useState('');

  const { data: approvals, isLoading } = useQuery({
    queryKey: ['approvals', releaseId],
    queryFn: () => approvalApi.list(releaseId!).then((r) => r.data),
    enabled: !!releaseId,
  });

  const vote = useMutation({
    mutationFn: (decision: 'approve' | 'reject') => approvalApi.vote(releaseId!, { decision, comment: comment || undefined }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['approvals', releaseId] }); setComment(''); },
  });

  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Approvals</h1>
      <Card>
        <CardHeader><CardTitle className="text-sm">Cast Vote</CardTitle></CardHeader>
        <CardContent className="flex gap-2">
          <input
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring max-w-sm"
            placeholder="Comment (optional)"
            value={comment}
            onChange={(e) => setComment(e.target.value)}
          />
          <Button variant="default" onClick={() => vote.mutate('approve')} disabled={vote.isPending}>Approve</Button>
          <Button variant="destructive" onClick={() => vote.mutate('reject')} disabled={vote.isPending}>Reject</Button>
        </CardContent>
      </Card>
      <Card>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Voter</TableHead>
                <TableHead>Decision</TableHead>
                <TableHead>Comment</TableHead>
                <TableHead>Date</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow><TableCell colSpan={4}>Loading...</TableCell></TableRow>
              ) : (
                approvals?.map((a: any) => (
                  <TableRow key={a.id}>
                    <TableCell className="font-medium">{a.voterName ?? a.voterId}</TableCell>
                    <TableCell>
                      <Badge variant={a.decision === 'approve' ? 'success' : 'destructive'}>{a.decision}</Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{a.comment ?? '-'}</TableCell>
                    <TableCell className="text-muted-foreground">{new Date(a.createdAt).toLocaleDateString()}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
