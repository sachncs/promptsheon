import Link from 'next/link';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Layers, FolderOpen, Activity, Bell } from 'lucide-react';

export default function NotFound() {
  return (
    <div className="flex h-full items-center justify-center p-6">
      <Card className="max-w-md">
        <CardHeader>
          <CardTitle>404 — Not Found</CardTitle>
          <CardDescription>The page you are looking for does not exist.</CardDescription>
        </CardHeader>
        <CardContent>
          <Link href="/">
            <Button>Back to Dashboard</Button>
          </Link>
        </CardContent>
      </Card>
    </div>
  );
}