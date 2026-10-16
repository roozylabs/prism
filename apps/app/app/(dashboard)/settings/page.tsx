import { redirect } from 'next/navigation';
import { AppRoutes } from '@/constants/routes';

// The old general settings page kept every field in local state and saved
// nothing; organization settings are stored from /settings/organization.
export default function SettingsPage() {
  redirect(AppRoutes.SETTINGS_ORGANIZATION);
}
