/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useNavigate } from '@tanstack/react-router'
import { ArrowRight, ShieldCheck, UserRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { AuthUser } from '@/stores/auth-store'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

type ProductFlowAuthorizeCardProps = {
  redirectTo: string
  user: AuthUser
}

export function ProductFlowAuthorizeCard({
  redirectTo,
  user,
}: ProductFlowAuthorizeCardProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const displayName =
    user.display_name || user.username || user.email || `#${user.id}`
  const secondary = user.email && user.email !== displayName ? user.email : null

  const handleAuthorize = () => {
    window.location.assign(redirectTo)
  }

  const handleCancel = () => {
    navigate({ to: '/dashboard', replace: true })
  }

  return (
    <div className='w-full space-y-8'>
      <div className='space-y-2 text-center sm:text-left'>
        <h2 className='text-2xl font-semibold tracking-tight'>
          {t('ProductFlow image workspace')}
        </h2>
        <p className='text-muted-foreground text-sm sm:text-base'>
          {t('Open the configured ProductFlow image workspace.')}
        </p>
      </div>

      <Card>
        <CardHeader className='border-b pb-4'>
          <div className='bg-primary/10 text-primary mb-3 flex h-11 w-11 items-center justify-center rounded-lg'>
            <ShieldCheck className='h-5 w-5' />
          </div>
          <CardTitle>{t('Authorize')}</CardTitle>
          <CardDescription>{t('ProductFlow image workspace')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-5'>
          <div className='bg-muted/40 flex items-center gap-3 rounded-xl border p-3'>
            <div className='bg-background flex h-10 w-10 shrink-0 items-center justify-center rounded-full border'>
              <UserRound className='h-4 w-4' />
            </div>
            <div className='min-w-0'>
              <div className='text-muted-foreground text-xs font-medium'>
                {t('Signed in')}
              </div>
              <div className='truncate text-sm font-medium'>{displayName}</div>
              {secondary ? (
                <div className='text-muted-foreground truncate text-xs'>
                  {secondary}
                </div>
              ) : null}
            </div>
          </div>

          <div className='flex flex-col gap-2 sm:flex-row'>
            <Button
              type='button'
              size='lg'
              className='flex-1 gap-2'
              onClick={handleAuthorize}
            >
              {t('Authorize')}
              <ArrowRight className='h-4 w-4' />
            </Button>
            <Button
              type='button'
              size='lg'
              variant='outline'
              className='sm:w-28'
              onClick={handleCancel}
            >
              {t('Cancel')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
