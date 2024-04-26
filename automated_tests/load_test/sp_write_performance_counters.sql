USE[master]
  GO
SET ANSI_NULLS
ON
  GO
SET QUOTED_IDENTIFIER
ON
  GO

/***
LogFile path has to be in a directory that SQL Server can Write To.
*/
CREATE
  PROCEDURE[dbo].[sp_write_performance_counters] @LogFile varchar(2000)
  = 'C:\\WINDOWS\\TEMP\\sqlPerf.log',
  @SecondsToRun int = 1600,
  @RunIntervalSeconds int = 2
AS BEGIN
  --File writing variables
  DECLARE @OACreate INT,
  @OAFile INT,
  @FileName VARCHAR(2000),
  @RowText VARCHAR(500),
  @Loops int,
  @LoopCounter int,
  @WaitForSeconds
    varchar(10)
      --Variables to save last counter values
      DECLARE @LastTPS BIGINT,
  @LastLRS BIGINT,
  @LastLTS BIGINT,
  @LastLWS BIGINT,
  @LastNDS BIGINT,
  @LastAWT BIGINT,
  @LastAWT_Base BIGINT,
  @LastALWT BIGINT,
  @LastALWT_Base
    BIGINT
      --Variables to save current counter values
      DECLARE @TPS BIGINT,
  @Active BIGINT,
  @SCM BIGINT,
  @LRS BIGINT,
  @LTS BIGINT,
  @LWS BIGINT,
  @NDS BIGINT,
  @AWT BIGINT,
  @AWT_Base BIGINT,
  @ALWT BIGINT,
  @ALWT_Base BIGINT,
  @ALWT_DIV BIGINT,
  @AWT_DIV BIGINT
SELECT
  @Loops = CASE
    WHEN (@SecondsToRun % @RunIntervalSeconds) > 5 THEN @SecondsToRun / @RunIntervalSeconds + 1
    ELSE @SecondsToRun / @RunIntervalSeconds
    END
SET @LoopCounter = 0
SELECT @WaitForSeconds = CONVERT(varchar, DATEADD(s, @RunIntervalSeconds, 0), 114)
SELECT
  @FileName
    = @LogFile
      + FORMAT(GETDATE(), '-MM-dd-yyyy_m', 'en-US')
      + '.txt'

        --Create the File Handler and Open the File
        EXECUTE sp_OACreate 'Scripting.FileSystemObject',
  @OACreate
    OUT
      EXECUTE sp_OAMethod @OACreate,
  'OpenTextFile',
  @OAFile OUT,
  @FileName,
  2,
  TRUE,
  -2

    --Write the Header
    EXECUTE sp_OAMethod @OAFile,
  'WriteLine',
  NULL,
  'Transactions/sec, Active Transactions, SQL Cache Memory (KB), Lock Requests/sec, Lock Timeouts/sec, Lock Waits/sec, Number of Deadlocks/sec, Average Wait Time (ms), Average Latch Wait Time (ms)'
--Collect Initial Sample Values
SET ANSI_WARNINGS OFF
SELECT
  @LastTPS = max(CASE WHEN counter_name = 'Transactions/sec' THEN cntr_value END),
  @LastLRS = max(CASE WHEN counter_name = 'Lock Requests/sec' THEN cntr_value END),
  @LastLTS = max(CASE WHEN counter_name = 'Lock Timeouts/sec' THEN cntr_value END),
  @LastLWS = max(CASE WHEN counter_name = 'Lock Waits/sec' THEN cntr_value END),
  @LastNDS = max(CASE WHEN counter_name = 'Number of Deadlocks/sec' THEN cntr_value END),
  @LastAWT = max(CASE WHEN counter_name = 'Average Wait Time (ms)' THEN cntr_value END),
  @LastAWT_Base = max(CASE WHEN counter_name = 'Average Wait Time base' THEN cntr_value END),
  @LastALWT = max(CASE WHEN counter_name = 'Average Latch Wait Time (ms)' THEN cntr_value END),
  @LastALWT_Base = max(CASE WHEN counter_name = 'Average Latch Wait Time base' THEN cntr_value END)
FROM sys.dm_os_performance_counters
WHERE
  counter_name IN (
    'Transactions/sec',
    'Lock Requests/sec',
    'Lock Timeouts/sec',
    'Lock Waits/sec',
    'Number of Deadlocks/sec',
    'Average Wait Time (ms)',
    'Average Wait Time base',
    'Average Latch Wait Time (ms)',
    'Average Latch Wait Time base')
  AND instance_name IN ('_Total', '')
SET ANSI_WARNINGS
ON
  WHILE @LoopCounter
  <= @Loops
    BEGIN
      WAITFOR DELAY @WaitForSeconds
SET ANSI_WARNINGS OFF
SELECT
  @TPS = max(CASE WHEN counter_name = 'Transactions/sec' THEN cntr_value END),
  @Active = max(CASE WHEN counter_name = 'Active Transactions' THEN cntr_value END),
  @SCM = max(CASE WHEN counter_name = 'SQL Cache Memory (KB)' THEN cntr_value END),
  @LRS = max(CASE WHEN counter_name = 'Lock Requests/sec' THEN cntr_value END),
  @LTS = max(CASE WHEN counter_name = 'Lock Timeouts/sec' THEN cntr_value END),
  @LWS = max(CASE WHEN counter_name = 'Lock Waits/sec' THEN cntr_value END),
  @NDS = max(CASE WHEN counter_name = 'Number of Deadlocks/sec' THEN cntr_value END),
  @AWT = max(CASE WHEN counter_name = 'Average Wait Time (ms)' THEN cntr_value END),
  @AWT_Base = max(CASE WHEN counter_name = 'Average Wait Time base' THEN cntr_value END),
  @ALWT = max(CASE WHEN counter_name = 'Average Latch Wait Time (ms)' THEN cntr_value END),
  @ALWT_Base = max(CASE WHEN counter_name = 'Average Latch Wait Time base' THEN cntr_value END)
FROM sys.dm_os_performance_counters
WHERE
  counter_name IN (
    'Transactions/sec',
    'Active Transactions',
    'SQL Cache Memory (KB)',
    'Lock Requests/sec',
    'Lock Timeouts/sec',
    'Lock Waits/sec',
    'Number of Deadlocks/sec',
    'Average Wait Time (ms)',
    'Average Wait Time base',
    'Average Latch Wait Time (ms)',
    'Average Latch Wait Time base')
  AND instance_name IN ('_Total', '')
SET ANSI_WARNINGS
ON
SELECT
  @AWT_DIV = CASE WHEN (@AWT_Base - @LastAWT_Base) > 0 THEN (@AWT_Base - @LastAWT_Base) ELSE 1 END,
  @ALWT_DIV
    = CASE WHEN (@ALWT_Base - @LastALWT_Base) > 0 THEN (@ALWT_Base - @LastALWT_Base) ELSE 1 END
SELECT
  @RowText = '' + convert(varchar, (@TPS - @LastTPS) / @RunIntervalSeconds) + ', '
    + convert(varchar, @Active) + ', ' + convert(varchar, @SCM) + ', '
    + convert(varchar, (@LRS - @LastLRS) / @RunIntervalSeconds)
    + ', '
    + convert(varchar, (@LTS - @LastLTS) / @RunIntervalSeconds)
    + ', ' + convert(varchar, (@LWS - @LastLWS) / @RunIntervalSeconds) + ', '
    + convert(varchar, (@NDS - @LastNDS) / @RunIntervalSeconds) + ', '
    + convert(varchar, (@AWT - @LastAWT) / @AWT_DIV)
    + ', '
    + convert(varchar, (@ALWT - @LastALWT) / @ALWT_DIV)
SELECT
  @LastTPS = @TPS,
  @LastLRS = @LRS,
  @LastLTS = @LTS,
  @LastLWS = @LWS,
  @LastNDS = @NDS,
  @LastAWT = @AWT,
  @LastAWT_Base = @AWT_Base,
  @LastALWT = @ALWT,
  @LastALWT_Base
    = @ALWT_Base
      EXECUTE sp_OAMethod @OAFile,
  'WriteLine',
  NULL,
  @RowText
SET @LoopCounter =
  @LoopCounter
  + 1
    END

      --CLEAN UP
      EXECUTE
        sp_OADestroy
          @OAFile
            EXECUTE sp_OADestroy @OACreate print 'Completed Logging Performance Metrics to file: '
  + @FileName
    END
      GO
