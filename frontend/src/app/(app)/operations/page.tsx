'use client';

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import Link from 'next/link';
import { Activity, Target, Bell, Settings } from 'lucide-react';

export default function OperationsPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-3xl font-bold">Operations Hub</h1>
      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="goals">Goals</TabsTrigger>
          <TabsTrigger value="alerts">Alerts</TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
        </TabsList>
        <TabsContent value="overview" className="mt-4 space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-sm">
                  <Activity className="h-4 w-4" />System Status
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Badge variant="success">All Systems Operational</Badge>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-sm">
                  <Target className="h-4 w-4" />Goals
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Link href="/goals" className="text-primary hover:underline">
                  View Goals Dashboard →
                </Link>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-sm">
                  <Bell className="h-4 w-4" />Alerts
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Link href="/alerts/active" className="text-primary hover:underline">
                  View Active Alerts →
                </Link>
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-sm">
                  <Settings className="h-4 w-4" />Settings
                </CardTitle>
              </CardHeader>
              <CardContent>
                <Link href="/settings" className="text-primary hover:underline">
                  View Settings →
                </Link>
              </CardContent>
            </Card>
          </div>
        </TabsContent>
        <TabsContent value="goals" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Goals</CardTitle>
              <CardDescription>Track active evolution goals.</CardDescription>
            </CardHeader>
            <CardContent>
              <Link href="/goals" className="text-primary hover:underline">
                Open Goals Dashboard →
              </Link>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="alerts" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">Alert Rules</CardTitle>
            </CardHeader>
            <CardContent>
              <Link href="/alerts/rules" className="text-primary hover:underline">
                Manage Rules →
              </Link>
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="settings" className="mt-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm">System Settings</CardTitle>
            </CardHeader>
            <CardContent>
              <Link href="/settings" className="text-primary hover:underline">
                Open Settings →
              </Link>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}