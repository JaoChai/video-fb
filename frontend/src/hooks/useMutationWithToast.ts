import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useToast } from '../components/ui/toaster';

interface Options<TData, TVariables> {
  mutationFn: (variables: TVariables) => Promise<TData>;
  invalidateKeys?: string[][];
  successMsg: string;
  onSuccess?: () => void;
}

export function useMutationWithToast<TData = unknown, TVariables = void>(
  opts: Options<TData, TVariables>,
) {
  const qc = useQueryClient();
  const { success, error: showError } = useToast();

  return useMutation({
    mutationFn: opts.mutationFn,
    onSuccess: () => {
      if (opts.invalidateKeys) {
        opts.invalidateKeys.forEach(key => qc.invalidateQueries({ queryKey: key }));
      }
      success(opts.successMsg);
      opts.onSuccess?.();
    },
    onError: (e: Error) => showError(`${opts.successMsg.replace('แล้ว', 'ล้มเหลว')}: ${e.message}`),
  });
}
