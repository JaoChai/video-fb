import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card';
import { Badge } from '../ui/badge';
import { Skeleton } from '../ui/skeleton';

interface ZernioAccount {
  _id: string;
  platform: string;
  displayName: string;
  username: string;
  profilePicture: string;
  profileUrl: string;
  followersCount: number;
  isActive: boolean;
  platformStatus: string;
  metadata?: {
    profileData?: {
      extraData?: {
        totalViews?: number;
        videoCount?: number;
      };
    };
  };
}

interface ConnectedAccountsCardProps {
  accounts: ZernioAccount[] | undefined;
  selectedId: string | undefined;
  loading: boolean;
}

export function ConnectedAccountsCard({ accounts, selectedId, loading }: ConnectedAccountsCardProps) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-3">
          <CardTitle className="text-base">Connected Accounts</CardTitle>
          <Badge variant="secondary" className="text-[10px]">via Zernio</Badge>
        </div>
        <CardDescription>YouTube and social media accounts connected through Zernio</CardDescription>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="space-y-3">
            {[1, 2].map(i => (
              <div key={i} className="flex items-center gap-3.5 rounded-lg border p-3.5">
                <Skeleton className="w-10 h-10 rounded-full" />
                <div className="flex-1 space-y-1.5">
                  <Skeleton className="h-4 w-32" />
                  <Skeleton className="h-3 w-48" />
                </div>
                <Skeleton className="h-5 w-20 rounded-full" />
              </div>
            ))}
          </div>
        ) : accounts?.length ? (
          <div className="grid gap-3">
            {accounts.filter(a => a._id === selectedId).map(account => {
              const extra = account.metadata?.profileData?.extraData;
              return (
                <div
                  key={account._id}
                  className="flex items-center gap-3.5 rounded-lg p-3.5 border bg-green-500/5 border-green-500/30"
                >
                  <img src={account.profilePicture} alt="" className="w-10 h-10 rounded-full" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{account.displayName}</span>
                      <Badge variant="secondary" className="text-[10px] bg-green-500/15 text-green-500 border-0">
                        Active
                      </Badge>
                    </div>
                    <div className="flex gap-3 mt-1">
                      <span className="text-[11px] text-muted-foreground">@{account.username}</span>
                      <span className="text-[11px] text-muted-foreground capitalize">{account.platform}</span>
                      {account.followersCount > 0 && (
                        <span className="text-[11px] text-muted-foreground">{account.followersCount.toLocaleString()} subs</span>
                      )}
                      {extra?.videoCount != null && (
                        <span className="text-[11px] text-muted-foreground">{extra.videoCount} videos</span>
                      )}
                    </div>
                  </div>
                  <Badge
                    variant={account.isActive ? 'secondary' : 'destructive'}
                    className={`text-[10px] shrink-0 ${
                      account.isActive
                        ? 'bg-green-500/10 text-green-500 border-0'
                        : 'bg-red-500/10 text-red-500 border-0'
                    }`}
                  >
                    {account.isActive ? 'Connected' : 'Disconnected'}
                  </Badge>
                </div>
              );
            })}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">
            No channels connected. Set Zernio API Key above and connect channels in Zernio dashboard.
          </p>
        )}
      </CardContent>
    </Card>
  );
}
