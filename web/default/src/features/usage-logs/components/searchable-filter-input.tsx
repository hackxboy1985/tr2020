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
import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ComboboxInput,
  type ComboboxInputOption,
} from '@/components/ui/combobox-input'

type FetchFn = (keyword: string) => Promise<ComboboxInputOption[]>

interface SearchableFilterInputProps {
  placeholder?: string
  value: string
  onChange: (value: string) => void
  fetchOptions: FetchFn
  className?: string
  /** Minimum characters before fetching (default 1) */
  minSearchLength?: number
  /** Debounce delay in ms (default 300) */
  debounceMs?: number
}

export function SearchableFilterInput({
  placeholder,
  value,
  onChange,
  fetchOptions,
  className,
  minSearchLength = 1,
  debounceMs = 300,
}: SearchableFilterInputProps) {
  const { t } = useTranslation()
  const [options, setOptions] = useState<ComboboxInputOption[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const debounceRef = useRef<ReturnType<typeof setTimeout>>()
  const lastKeywordRef = useRef('')

  const fetchAndSetOptions = useCallback(
    async (keyword: string) => {
      if (keyword.length < minSearchLength) {
        setOptions([])
        return
      }
      lastKeywordRef.current = keyword
      setIsLoading(true)
      try {
        const result = await fetchOptions(keyword)
        // Avoid stale responses
        if (lastKeywordRef.current === keyword) {
          setOptions(result)
        }
      } catch {
        if (lastKeywordRef.current === keyword) {
          setOptions([])
        }
      } finally {
        if (lastKeywordRef.current === keyword) {
          setIsLoading(false)
        }
      }
    },
    [fetchOptions, minSearchLength]
  )

  const handleSearchChange = useCallback(
    (searchText: string) => {
      if (debounceRef.current) {
        clearTimeout(debounceRef.current)
      }
      // Update the filter value immediately so the input is responsive
      onChange(searchText)
      debounceRef.current = setTimeout(() => {
        fetchAndSetOptions(searchText)
      }, debounceMs)
    },
    [debounceMs, fetchAndSetOptions, onChange]
  )

  // Cleanup debounce on unmount
  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [])

  const emptyText = useMemo(
    () => (isLoading ? t('Searching...') : t('No matching results')),
    [isLoading, t]
  )

  return (
    <ComboboxInput
      options={options}
      value={value}
      onValueChange={handleSearchChange}
      placeholder={placeholder}
      emptyText={emptyText}
      className={className}
      allowCustomValue
    />
  )
}
